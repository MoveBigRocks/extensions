// Package store persists operational state only in the extension-owned schema.
package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/movebigrocks/extension-sdk/extdb"
	"github.com/movebigrocks/extension-sdk/runtimehost"
	"github.com/movebigrocks/extensions/operational-health/domain"
)

const Schema = "ext_demandops_operational_health"
const incidentColumns = `id,workspace_id AS workspaceid,source_id AS sourceid,environment,component,severity,
 COALESCE(case_id::text,'') AS caseid,case_status AS casestatus,signal_state AS signalstate,
 owner_id AS ownerid,runbook_id AS runbookid,canary,revision,created_at AS createdat,updated_at AS updatedat,closed_at AS closedat`

type Store struct{ DB *extdb.DB }

func New(db *extdb.DB) *Store { return &Store{DB: db} }
func query(s string) string   { return strings.ReplaceAll(s, "{s}", Schema) }
func (s *Store) scoped(ctx context.Context, workspace string, fn func(context.Context) error) error {
	if _, err := uuid.Parse(workspace); err != nil {
		return domain.ErrForbidden
	}
	return s.DB.Transaction(ctx, func(tx context.Context) error {
		if _, err := s.DB.Get(tx).ExecContext(tx, `SELECT set_config('app.current_workspace_id',?,true)`, workspace); err != nil {
			return err
		}
		return fn(tx)
	})
}
func (s *Store) Health(ctx context.Context) error {
	var n int
	return s.DB.Get(ctx).GetContext(ctx, &n, query(`SELECT count(*) FROM {s}.sources`))
}
func (s *Store) syncSource(ctx context.Context, workspace string, src domain.Source, now time.Time) error {
	digest := domain.Digest(src)
	var stored struct {
		Digest      string `db:"configuration_digest"`
		Environment string
	}
	err := s.DB.Get(ctx).GetContext(ctx, &stored, query(`SELECT configuration_digest,environment FROM {s}.sources WHERE workspace_id=? AND id=?`), workspace, src.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if stored.Environment != "" && stored.Environment != src.Environment {
		return domain.ErrConflict
	}
	if stored.Digest == digest {
		return nil
	}
	_, err = s.DB.Get(ctx).ExecContext(ctx, query(`INSERT INTO {s}.sources(id,workspace_id,environment,configuration_digest,updated_at) VALUES(?,?,?,?,?)
 ON CONFLICT(id) DO UPDATE SET configuration_digest=EXCLUDED.configuration_digest,updated_at=EXCLUDED.updated_at WHERE {s}.sources.workspace_id=EXCLUDED.workspace_id`), src.ID, workspace, src.Environment, digest, now)
	if err != nil {
		return err
	}
	_, err = s.DB.Get(ctx).ExecContext(ctx, query(`INSERT INTO {s}.configuration_events(workspace_id,source_id,digest,created_at) VALUES(?,?,?,?)`), workspace, src.ID, digest, now)
	return err
}

type Receipt struct {
	ID        string `json:"id"`
	Duplicate bool   `json:"duplicate"`
}
type ProjectionPayload struct {
	Create *runtimehost.CreateCaseInput `json:"create,omitempty"`
	Body   string                       `json:"body,omitempty"`
}

func (s *Store) enqueue(ctx context.Context, ws, id, key, kind string, payload ProjectionPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.DB.Get(ctx).ExecContext(ctx, query(`INSERT INTO {s}.projection_outbox(workspace_id,incident_id,operation_key,kind,payload) VALUES(?,?,?,?,?::jsonb) ON CONFLICT(workspace_id,operation_key) DO NOTHING`), ws, id, key, kind, string(data))
	return err
}
func (s *Store) Ingest(ctx context.Context, workspace string, cfg domain.Config, src domain.Source, signals []domain.Signal, truncated int, canary bool, now time.Time) (Receipt, error) {
	var receipt Receipt
	err := s.scoped(ctx, workspace, func(tx context.Context) error {
		if _, err := s.DB.Get(tx).ExecContext(tx, `SELECT pg_advisory_xact_lock(hashtextextended(?,0))`, workspace+"/"+src.ID); err != nil {
			return err
		}
		if err := s.syncSource(tx, workspace, src, now); err != nil {
			return err
		}
		if _, err := s.DB.Get(tx).ExecContext(tx, query(`UPDATE {s}.sources SET last_delivery_at=?,last_canary_at=CASE WHEN ? THEN ? ELSE last_canary_at END WHERE workspace_id=? AND id=?`), now, canary, now, workspace, src.ID); err != nil {
			return err
		}
		digest := domain.Digest(struct {
			Signals   []domain.Signal
			Truncated int
			Canary    bool
		}{signals, truncated, canary})
		err := s.DB.Get(tx).GetContext(tx, &receipt.ID, query(`INSERT INTO {s}.deliveries(workspace_id,source_id,digest,truncated_alerts,received_at) VALUES(?,?,?,?,?) ON CONFLICT(workspace_id,source_id,digest) DO NOTHING RETURNING id`), workspace, src.ID, digest, truncated, now)
		if errors.Is(err, sql.ErrNoRows) {
			receipt.Duplicate = true
			return s.DB.Get(tx).GetContext(tx, &receipt.ID, query(`SELECT id FROM {s}.deliveries WHERE workspace_id=? AND source_id=? AND digest=?`), workspace, src.ID, digest)
		}
		if err != nil {
			return err
		}
		for _, signal := range signals {
			if err := s.applySignal(tx, workspace, cfg, src, signal, now); err != nil {
				return err
			}
		}
		return nil
	})
	return receipt, err
}
func (s *Store) applySignal(ctx context.Context, ws string, cfg domain.Config, src domain.Source, in domain.Signal, now time.Time) error {
	var old struct {
		Status     string
		Component  string
		IncidentID sql.NullString `db:"incident_id"`
	}
	err := s.DB.Get(ctx).GetContext(ctx, &old, query(`SELECT status,component,incident_id FROM {s}.signals WHERE workspace_id=? AND source_id=? AND fingerprint=? AND starts_at=? FOR UPDATE`), ws, src.ID, in.Fingerprint, in.StartsAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if old.Component != "" && old.Component != in.Component {
		return domain.ErrConflict
	}
	next := domain.Transition(old.Status, in.Status)
	incidentID := old.IncidentID.String
	if incidentID == "" && next == "firing" {
		err = s.DB.Get(ctx).GetContext(ctx, &incidentID, query(`SELECT id FROM {s}.incidents WHERE workspace_id=? AND source_id=? AND component=? AND closed_at IS NULL FOR UPDATE`), ws, src.ID, in.Component)
		if errors.Is(err, sql.ErrNoRows) {
			target := src.Components[in.Component]
			err = s.DB.Get(ctx).GetContext(ctx, &incidentID, query(`INSERT INTO {s}.incidents(workspace_id,source_id,environment,component,severity,owner_id,runbook_id,canary,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?) RETURNING id`), ws, src.ID, src.Environment, in.Component, in.Severity, target.OwnerID, target.RunbookID, src.Canary, now, now)
			if err != nil {
				return err
			}
			priority := "normal"
			if in.Severity == "critical" {
				priority = "urgent"
			}
			create := runtimehost.CreateCaseInput{WorkspaceID: ws, IdempotencyKey: "incident/" + incidentID, Subject: fmt.Sprintf("[%s] %s: %s", src.Environment, in.Component, in.AlertName), Description: "Operational incident. Follow linked evidence and verify current health before closure.", Priority: priority, Channel: "api", Category: "incident", QueueID: cfg.QueueID, AssignedToID: target.OwnerID, Tags: []string{"operational-health", src.Environment, in.Component}, CustomFields: map[string]any{"incident_id": incidentID, "source_id": src.ID, "environment": src.Environment, "component": in.Component, "canary": src.Canary}}
			if err = s.enqueue(ctx, ws, incidentID, create.IdempotencyKey, "create", ProjectionPayload{Create: &create}); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	var linked any
	if incidentID != "" {
		linked = incidentID
	}
	_, err = s.DB.Get(ctx).ExecContext(ctx, query(`INSERT INTO {s}.signals(workspace_id,source_id,fingerprint,starts_at,ends_at,incident_id,component,alert_name,severity,status,received_at)
 VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(workspace_id,source_id,fingerprint,starts_at) DO UPDATE SET status=EXCLUDED.status,ends_at=COALESCE({s}.signals.ends_at,EXCLUDED.ends_at),received_at=EXCLUDED.received_at,incident_id=COALESCE({s}.signals.incident_id,EXCLUDED.incident_id)`), ws, src.ID, in.Fingerprint, in.StartsAt, in.EndsAt, linked, in.Component, in.AlertName, in.Severity, next, now)
	if err != nil {
		return err
	}
	if incidentID != "" {
		_, err = s.DB.Get(ctx).ExecContext(ctx, query(`UPDATE {s}.incidents SET signal_state=CASE WHEN EXISTS(SELECT 1 FROM {s}.signals WHERE workspace_id=? AND incident_id=? AND status='firing') THEN 'firing' WHEN EXISTS(SELECT 1 FROM {s}.signals WHERE workspace_id=? AND incident_id=? AND status='unknown') THEN 'unknown' ELSE 'recovered' END,
 severity=CASE WHEN severity='critical' OR ?='critical' THEN 'critical' ELSE severity END,updated_at=?,revision=revision+1 WHERE workspace_id=? AND id=?`), ws, incidentID, ws, incidentID, in.Severity, now, ws, incidentID)
		if err != nil {
			return err
		}
		if old.Status != next {
			key := "signal/" + domain.Digest(struct {
				ID          string
				Fingerprint string
				Start       time.Time
				State       string
			}{incidentID, in.Fingerprint, in.StartsAt, next})
			return s.enqueue(ctx, ws, incidentID, key, "note", ProjectionPayload{Body: fmt.Sprintf("%s %s. Source event started %s; received %s. Environment: %s. Component: %s.", in.AlertName, next, in.StartsAt.Format(time.RFC3339), now.Format(time.RFC3339), src.Environment, in.Component)})
		}
	}
	return nil
}
func (s *Store) Observe(ctx context.Context, ws string, src domain.Source, observations []domain.Observation, now time.Time) error {
	return s.scoped(ctx, ws, func(tx context.Context) error {
		if err := s.syncSource(tx, ws, src, now); err != nil {
			return err
		}
		for _, o := range observations {
			body, err := json.Marshal(o)
			if err != nil {
				return err
			}
			_, err = s.DB.Get(tx).ExecContext(tx, query(`INSERT INTO {s}.observations(workspace_id,source_id,component,observed_at,received_at,payload) VALUES(?,?,?,?,?,?::jsonb) ON CONFLICT(workspace_id,source_id,component,observed_at) DO NOTHING`), ws, src.ID, o.Component, o.ObservedAt, now, string(body))
			if err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Store) Deploy(ctx context.Context, ws string, src domain.Source, d domain.Deployment, now time.Time) error {
	return s.scoped(ctx, ws, func(tx context.Context) error {
		body, err := json.Marshal(d)
		if err != nil {
			return err
		}
		_, err = s.DB.Get(tx).ExecContext(tx, query(`INSERT INTO {s}.deployments(workspace_id,source_id,component,run_id,starts_at,expires_at,state,payload,received_at) VALUES(?,?,?,?,?,?,?,?::jsonb,?) ON CONFLICT(workspace_id,source_id,component,run_id,state) DO NOTHING`), ws, src.ID, d.Component, d.RunID, d.StartsAt, d.ExpiresAt, d.State, string(body), now)
		return err
	})
}

type Filter = domain.Filter

type cursor struct {
	Workspace string    `json:"w"`
	Created   time.Time `json:"t"`
	ID        string    `json:"i"`
	Scope     string    `json:"s"`
}

func filterScope(f Filter) string { f.After = ""; f.Limit = 0; return domain.Digest(f) }
func (s *Store) List(ctx context.Context, ws string, f Filter) (domain.Page, error) {
	out := domain.Page{Items: []domain.Incident{}}
	if f.Limit < 1 || f.Limit > 100 || len(f.After) > 1024 {
		return out, domain.ErrInvalid
	}
	var cur cursor
	if f.After != "" {
		data, err := base64.RawURLEncoding.DecodeString(f.After)
		if err != nil || json.Unmarshal(data, &cur) != nil || cur.Workspace != ws || cur.Scope != filterScope(f) {
			return out, domain.ErrInvalid
		}
		if _, err = uuid.Parse(cur.ID); err != nil {
			return out, domain.ErrInvalid
		}
	}
	err := s.scoped(ctx, ws, func(tx context.Context) error {
		q := `SELECT ` + incidentColumns + ` FROM {s}.incidents WHERE workspace_id=?`
		args := []any{ws}
		for _, term := range []struct{ Column, Value string }{{"environment", f.Environment}, {"component", f.Component}, {"signal_state", f.State}, {"severity", f.Severity}, {"owner_id", f.Owner}} {
			if term.Value != "" {
				q += " AND " + term.Column + "=?"
				args = append(args, term.Value)
			}
		}
		if f.After != "" {
			q += " AND (created_at,id)<(?,?::uuid)"
			args = append(args, cur.Created, cur.ID)
		}
		q += " ORDER BY created_at DESC,id DESC LIMIT ?"
		args = append(args, f.Limit+1)
		return s.DB.Get(tx).SelectContext(tx, &out.Items, query(q), args...)
	})
	if err != nil {
		return out, err
	}
	if len(out.Items) > f.Limit {
		out.Items = out.Items[:f.Limit]
		last := out.Items[len(out.Items)-1]
		data, _ := json.Marshal(cursor{Workspace: ws, Created: last.CreatedAt, ID: last.ID, Scope: filterScope(f)})
		out.Next = base64.RawURLEncoding.EncodeToString(data)
	}
	return out, nil
}
func (s *Store) Get(ctx context.Context, ws, id string) (domain.Incident, error) {
	var out domain.Incident
	if _, err := uuid.Parse(id); err != nil {
		return out, domain.ErrInvalid
	}
	err := s.scoped(ctx, ws, func(tx context.Context) error {
		return s.DB.Get(tx).GetContext(tx, &out, query(`SELECT `+incidentColumns+` FROM {s}.incidents WHERE workspace_id=? AND id=?`), ws, id)
	})
	if errors.Is(err, sql.ErrNoRows) {
		err = domain.ErrNotFound
	}
	return out, err
}
func (s *Store) Status(ctx context.Context, ws string, cfg domain.Config, now time.Time) ([]domain.TargetStatus, error) {
	result := []domain.TargetStatus{}
	err := s.scoped(ctx, ws, func(tx context.Context) error {
		for _, src := range cfg.Sources {
			if !src.Enabled {
				continue
			}
			for name, target := range src.Components {
				item := domain.TargetStatus{SourceID: src.ID, Environment: src.Environment, Component: name, State: "unknown", DeliveryState: "unknown", Canary: src.Canary, OwnerID: target.OwnerID, RunbookID: target.RunbookID, DeploymentState: "none"}
				var last struct {
					Delivery *time.Time `db:"last_delivery_at"`
					Canary   *time.Time `db:"last_canary_at"`
					Snapshot *time.Time `db:"last_snapshot_at"`
				}
				err := s.DB.Get(tx).GetContext(tx, &last, query(`SELECT last_delivery_at,last_canary_at,last_snapshot_at FROM {s}.sources WHERE workspace_id=? AND id=?`), ws, src.ID)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return err
				}
				item.LastDelivery = last.Delivery
				item.LastCanary = last.Canary
				item.LastSnapshot = last.Snapshot
				if last.Canary != nil && now.Sub(*last.Canary) <= domain.FreshFor {
					item.DeliveryState = "healthy"
				}
				var body []byte
				err = s.DB.Get(tx).GetContext(tx, &body, query(`SELECT payload FROM {s}.observations WHERE workspace_id=? AND source_id=? AND component=? ORDER BY observed_at DESC LIMIT 1`), ws, src.ID, name)
				if err == nil {
					var o domain.Observation
					if err = json.Unmarshal(body, &o); err != nil {
						return err
					}
					item.Observation = &o
					expiry := o.ObservedAt.Add(domain.FreshFor)
					item.ExpiresAt = &expiry
					item.State = o.State(target, now)
				} else if !errors.Is(err, sql.ErrNoRows) {
					return err
				}
				if err = s.DB.Get(tx).GetContext(tx, &item.ActiveSignals, query(`SELECT count(*) FROM {s}.signals WHERE workspace_id=? AND source_id=? AND component=? AND status='firing'`), ws, src.ID, name); err != nil {
					return err
				}
				if err = s.DB.Get(tx).GetContext(tx, &item.PendingProjections, query(`SELECT count(*) FROM {s}.projection_outbox o JOIN {s}.incidents i ON i.id=o.incident_id WHERE o.workspace_id=? AND i.source_id=? AND i.component=? AND o.state<>'delivered'`), ws, src.ID, name); err != nil {
					return err
				}
				var deployment struct {
					RunID   string `db:"run_id"`
					State   string
					Expires time.Time `db:"expires_at"`
				}
				err = s.DB.Get(tx).GetContext(tx, &deployment, query(`SELECT run_id,state,expires_at FROM {s}.deployments WHERE workspace_id=? AND source_id=? AND component=? ORDER BY starts_at DESC, CASE state WHEN 'failed' THEN 0 WHEN 'completed' THEN 1 ELSE 2 END LIMIT 1`), ws, src.ID, name)
				if err == nil {
					item.DeploymentRunID = deployment.RunID
					item.DeploymentState = deployment.State
					if deployment.State == "started" {
						if now.Before(deployment.Expires) {
							item.MaintenanceUntil = &deployment.Expires
						} else {
							item.DeploymentState = "expired"
						}
					}
					if item.DeploymentState == "failed" || item.DeploymentState == "expired" {
						item.State = "unhealthy"
					}
				} else if !errors.Is(err, sql.ErrNoRows) {
					return err
				}
				if item.ActiveSignals > 0 {
					item.State = "unhealthy"
				} else if item.State != "unhealthy" && (item.DeliveryState != "healthy" || item.PendingProjections > 0) {
					item.State = "unknown"
				}
				result = append(result, item)
			}
		}
		return nil
	})
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Environment != b.Environment {
			return a.Environment < b.Environment
		}
		if a.SourceID != b.SourceID {
			return a.SourceID < b.SourceID
		}
		return a.Component < b.Component
	})
	return result, err
}
