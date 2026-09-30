package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/movebigrocks/extensions/operational-health/domain"
)

func evidenceCursor(ws, id, kind, after string) (string, error) {
	if after == "" {
		return "", nil
	}
	if len(after) > 1024 {
		return "", domain.ErrInvalid
	}
	var c struct{ Workspace, Incident, Kind, ID string }
	b, err := base64.RawURLEncoding.DecodeString(after)
	if err != nil || json.Unmarshal(b, &c) != nil || c.Workspace != ws || c.Incident != id || c.Kind != kind {
		return "", domain.ErrInvalid
	}
	if _, err = uuid.Parse(c.ID); err != nil {
		return "", domain.ErrInvalid
	}
	return c.ID, nil
}
func nextEvidence(ws, id, kind, last string) string {
	b, _ := json.Marshal(struct{ Workspace, Incident, Kind, ID string }{ws, id, kind, last})
	return base64.RawURLEncoding.EncodeToString(b)
}
func (s *Store) Evidence(ctx context.Context, ws, id string, f domain.EvidenceFilter) (domain.Evidence, error) {
	out := domain.Evidence{Signals: domain.SignalPage{Items: []domain.SignalEvidence{}}, Projections: domain.ProjectionPage{Items: []domain.ProjectionEvidence{}}}
	if f.Limit < 1 || f.Limit > 100 {
		return out, domain.ErrInvalid
	}
	if _, err := s.Get(ctx, ws, id); err != nil {
		return out, err
	}
	sig, err := evidenceCursor(ws, id, "signals", f.SignalsAfter)
	if err != nil {
		return out, err
	}
	proj, err := evidenceCursor(ws, id, "projections", f.ProjectionsAfter)
	if err != nil {
		return out, err
	}
	err = s.scoped(ctx, ws, func(tx context.Context) error {
		q := `SELECT id,component,alert_name AS alertname,severity,fingerprint,starts_at AS startsat,ends_at AS endsat,status,received_at AS receivedat FROM {s}.signals WHERE workspace_id=? AND incident_id=?`
		args := []any{ws, id}
		if sig != "" {
			q += ` AND id<?::uuid`
			args = append(args, sig)
		}
		q += ` ORDER BY id DESC LIMIT ?`
		args = append(args, f.Limit+1)
		if err := s.DB.Get(tx).SelectContext(tx, &out.Signals.Items, query(q), args...); err != nil {
			return err
		}
		q = `SELECT id,kind,state,attempts,last_error AS lasterror,created_at AS createdat,retry_at AS retryat,delivered_at AS deliveredat FROM {s}.projection_outbox WHERE workspace_id=? AND incident_id=?`
		args = []any{ws, id}
		if proj != "" {
			q += ` AND id<?::uuid`
			args = append(args, proj)
		}
		q += ` ORDER BY id DESC LIMIT ?`
		args = append(args, f.Limit+1)
		return s.DB.Get(tx).SelectContext(tx, &out.Projections.Items, query(q), args...)
	})
	if len(out.Signals.Items) > f.Limit {
		out.Signals.Items = out.Signals.Items[:f.Limit]
		out.Signals.Next = nextEvidence(ws, id, "signals", out.Signals.Items[f.Limit-1].ID)
	}
	if len(out.Projections.Items) > f.Limit {
		out.Projections.Items = out.Projections.Items[:f.Limit]
		out.Projections.Next = nextEvidence(ws, id, "projections", out.Projections.Items[f.Limit-1].ID)
	}
	return out, err
}
