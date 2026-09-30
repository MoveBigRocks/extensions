package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/movebigrocks/extensions/operational-health/domain"
)

func (s *Store) Snapshot(ctx context.Context, ws string, cfg domain.Config, src domain.Source, signals []domain.Signal, observed, now time.Time) error {
	return s.scoped(ctx, ws, func(tx context.Context) error {
		if _, err := s.DB.Get(tx).ExecContext(tx, `SELECT pg_advisory_xact_lock(hashtextextended(?,0))`, ws+"/"+src.ID); err != nil {
			return err
		}
		if err := s.syncSource(tx, ws, src, now); err != nil {
			return err
		}
		var previous *time.Time
		if err := s.DB.Get(tx).GetContext(tx, &previous, query(`SELECT last_snapshot_at FROM {s}.sources WHERE workspace_id=? AND id=?`), ws, src.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if previous != nil && !observed.After(*previous) {
			return nil
		}
		// First apply the positively observed episodes using the same correlation and
		// outbox transaction as webhooks. A complete empty snapshot is meaningful.
		seen := map[string]bool{}
		for _, sig := range signals {
			if err := s.applySignal(tx, ws, cfg, src, sig, now); err != nil {
				return err
			}
			seen[sig.Fingerprint+"/"+sig.StartsAt.Format(time.RFC3339Nano)] = true
		}
		var active []struct {
			ID          string
			Fingerprint string
			Start       time.Time `db:"starts_at"`
			IncidentID  string    `db:"incident_id"`
		}
		if err := s.DB.Get(tx).SelectContext(tx, &active, query(`SELECT id,fingerprint,starts_at,COALESCE(incident_id::text,'') AS incident_id FROM {s}.signals WHERE workspace_id=? AND source_id=? AND status='firing' AND starts_at<=? AND received_at<=?`), ws, src.ID, observed, observed); err != nil {
			return err
		}
		for _, sig := range active {
			if seen[sig.Fingerprint+"/"+sig.Start.Format(time.RFC3339Nano)] {
				continue
			}
			if _, err := s.DB.Get(tx).ExecContext(tx, query(`UPDATE {s}.signals SET status='unknown' WHERE workspace_id=? AND id=?`), ws, sig.ID); err != nil {
				return err
			}
			if sig.IncidentID != "" {
				if err := s.enqueue(tx, ws, sig.IncidentID, "snapshot/"+sig.ID+"/"+observed.Format(time.RFC3339Nano), "note", ProjectionPayload{Body: "A complete current Alertmanager snapshot no longer includes this episode. The recovery transition was not received; its end time remains unknown. Verify fresh checks before closure."}); err != nil {
					return err
				}
			}
		}
		if _, err := s.DB.Get(tx).ExecContext(tx, query(`UPDATE {s}.incidents i SET signal_state=CASE WHEN EXISTS(SELECT 1 FROM {s}.signals s WHERE s.workspace_id=i.workspace_id AND s.incident_id=i.id AND s.status='firing') THEN 'firing' WHEN EXISTS(SELECT 1 FROM {s}.signals s WHERE s.workspace_id=i.workspace_id AND s.incident_id=i.id AND s.status='unknown') THEN 'unknown' ELSE 'recovered' END,updated_at=?,revision=revision+1 WHERE workspace_id=? AND source_id=? AND closed_at IS NULL`), now, ws, src.ID); err != nil {
			return err
		}
		_, err := s.DB.Get(tx).ExecContext(tx, query(`UPDATE {s}.sources SET last_snapshot_at=? WHERE workspace_id=? AND id=?`), observed, ws, src.ID)
		return err
	})
}
