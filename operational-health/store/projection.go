package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/movebigrocks/extension-sdk/runtimehost"
	"github.com/movebigrocks/extensions/operational-health/domain"
)

type Projection struct {
	ID         string
	IncidentID string `db:"incident_id"`
	CaseID     string `db:"case_id"`
	Key        string `db:"operation_key"`
	Kind       string
	Payload    []byte
	Token      string `db:"claim_token"`
	Attempts   int
}

func (s *Store) Claim(ctx context.Context, ws string, now time.Time) (*Projection, error) {
	var out Projection
	err := s.scoped(ctx, ws, func(tx context.Context) error {
		err := s.DB.Get(tx).GetContext(tx, &out, query(`SELECT o.id,o.incident_id,COALESCE(i.case_id::text,'') AS case_id,o.operation_key,o.kind,o.payload,'' AS claim_token,o.attempts
 FROM {s}.projection_outbox o JOIN {s}.incidents i ON i.id=o.incident_id AND i.workspace_id=o.workspace_id
 WHERE o.workspace_id=? AND ((o.state='pending' AND o.retry_at<=?) OR (o.state='leased' AND o.lease_until<=?)) AND (o.kind='create' OR i.case_id IS NOT NULL)
 ORDER BY o.created_at,o.id LIMIT 1 FOR UPDATE OF o SKIP LOCKED`), ws, now, now)
		if err != nil {
			return err
		}
		out.Token = uuid.NewString()
		out.Attempts++
		_, err = s.DB.Get(tx).ExecContext(tx, query(`UPDATE {s}.projection_outbox SET state='leased',claim_token=?,lease_until=?,attempts=attempts+1 WHERE workspace_id=? AND id=?`), out.Token, now.Add(time.Minute), ws, out.ID)
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}
func (s *Store) Complete(ctx context.Context, ws string, p Projection, caseID string, now time.Time) error {
	return s.scoped(ctx, ws, func(tx context.Context) error {
		result, err := s.DB.Get(tx).ExecContext(tx, query(`UPDATE {s}.projection_outbox SET state='delivered',delivered_at=?,claim_token=NULL,lease_until=NULL,last_error='' WHERE workspace_id=? AND id=? AND state='leased' AND claim_token=?`), now, ws, p.ID, p.Token)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return domain.ErrConflict
		}
		if p.Kind == "create" {
			_, err = s.DB.Get(tx).ExecContext(tx, query(`UPDATE {s}.incidents SET case_id=?,case_status='open',updated_at=?,revision=revision+1 WHERE workspace_id=? AND id=? AND (case_id IS NULL OR case_id=?)`), caseID, now, ws, p.IncidentID, caseID)
		}
		return err
	})
}
func (s *Store) Fail(ctx context.Context, ws string, p Projection, code int, now time.Time) error {
	state := "pending"
	reason := "host_unavailable"
	if (code >= 400 && code < 500 && code != 408 && code != 429) || p.Attempts >= 12 {
		state = "quarantined"
		reason = "host_rejected_or_retry_exhausted"
	}
	delay := time.Duration(min(p.Attempts*p.Attempts, 300)) * time.Second
	return s.scoped(ctx, ws, func(tx context.Context) error {
		_, err := s.DB.Get(tx).ExecContext(tx, query(`UPDATE {s}.projection_outbox SET state=?,retry_at=?,last_error=?,claim_token=NULL,lease_until=NULL WHERE workspace_id=? AND id=? AND state='leased' AND claim_token=?`), state, now.Add(delay), reason, ws, p.ID, p.Token)
		return err
	})
}
func (s *Store) SyncCase(ctx context.Context, ws, id string, c runtimehost.HostCase, now time.Time) error {
	if c.WorkspaceID != ws {
		return domain.ErrForbidden
	}
	return s.scoped(ctx, ws, func(tx context.Context) error {
		_, err := s.DB.Get(tx).ExecContext(tx, query(`UPDATE {s}.incidents SET owner_id=?,case_status=?,closed_at=CASE WHEN ? IN ('resolved','closed') THEN COALESCE(closed_at,?) ELSE NULL END,updated_at=?,revision=revision+1 WHERE workspace_id=? AND id=? AND case_id=? AND (case_status<>? OR owner_id<>?)`), c.AssignedToID, c.Status, c.Status, now, now, ws, id, c.ID, c.Status, c.AssignedToID)
		if err != nil {
			return err
		}
		_, err = s.DB.Get(tx).ExecContext(tx, query(`UPDATE {s}.incidents SET last_case_check_at=? WHERE workspace_id=? AND id=? AND case_id=?`), now, ws, id, c.ID)
		return err
	})
}
func (s *Store) CasesToSync(ctx context.Context, ws string) ([]domain.Incident, error) {
	out := []domain.Incident{}
	err := s.scoped(ctx, ws, func(tx context.Context) error {
		return s.DB.Get(tx).SelectContext(tx, &out, query(`SELECT `+incidentColumns+` FROM {s}.incidents WHERE workspace_id=? AND case_id IS NOT NULL AND closed_at IS NULL ORDER BY last_case_check_at NULLS FIRST,id LIMIT 40`), ws)
	})
	return out, err
}
func (s *Store) SyncConfigured(ctx context.Context, ws string, cfg domain.Config, now time.Time) error {
	return s.scoped(ctx, ws, func(tx context.Context) error {
		for _, src := range cfg.Sources {
			if err := s.syncSource(tx, ws, src, now); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Store) Cleanup(ctx context.Context, ws string, now time.Time) error {
	return s.scoped(ctx, ws, func(tx context.Context) error {
		// Retain incident/episode/outbox tombstones. Only bounded, superseded telemetry expires.
		_, err := s.DB.Get(tx).ExecContext(tx, query(`DELETE FROM {s}.observations WHERE id IN (SELECT o.id FROM {s}.observations o WHERE o.workspace_id=? AND o.received_at<? AND EXISTS(SELECT 1 FROM {s}.observations newer WHERE newer.workspace_id=o.workspace_id AND newer.source_id=o.source_id AND newer.component=o.component AND newer.observed_at>o.observed_at) LIMIT 1000)`), ws, now.Add(-7*24*time.Hour))
		return err
	})
}
func (s *Store) Redrive(ctx context.Context, ws, id, actor string, now time.Time) error {
	if _, err := uuid.Parse(actor); err != nil {
		return domain.ErrForbidden
	}
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrInvalid
	}
	return s.scoped(ctx, ws, func(tx context.Context) error {
		var incidentID string
		err := s.DB.Get(tx).GetContext(tx, &incidentID, query(`UPDATE {s}.projection_outbox SET state='pending',attempts=0,retry_at=?,last_error='' WHERE workspace_id=? AND id=? AND state='quarantined' RETURNING incident_id`), now, ws, id)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		return s.enqueue(tx, ws, incidentID, "redrive/"+id+"/"+now.Format(time.RFC3339Nano), "note", ProjectionPayload{Body: "Operator " + actor + " retried quarantined incident projection " + id + "."})
	})
}
func DecodeProjection(p Projection) (ProjectionPayload, error) {
	var out ProjectionPayload
	err := json.Unmarshal(p.Payload, &out)
	return out, err
}
