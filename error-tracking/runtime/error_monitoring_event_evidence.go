package sql

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	observabilitydomain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	storecontracts "github.com/movebigrocks/extensions/error-tracking/runtime/storecontracts"
)

// eventEvidenceColumns deliberately omits message, exception, user, request,
// extra, contexts and breadcrumbs: evidence pages never load private payloads.
const eventEvidenceColumns = `id::text AS id, COALESCE(event_id, '') AS event_id, project_id::text AS project_id,
	COALESCE(issue_id::text, '') AS issue_id, timestamp, COALESCE(environment, '') AS environment,
	COALESCE(release, '') AS release, tags`

// tagLookupStatementTimeout caps a tag lookup even when a common value matches
// many rows; callers receive an error instead of holding a connection.
const tagLookupStatementTimeout = "5s"

type eventEvidenceRow struct {
	ID          string    `db:"id"`
	EventID     string    `db:"event_id"`
	ProjectID   string    `db:"project_id"`
	IssueID     string    `db:"issue_id"`
	Timestamp   time.Time `db:"timestamp"`
	Environment string    `db:"environment"`
	Release     string    `db:"release"`
	Tags        []byte    `db:"tags"`
}

func (r eventEvidenceRow) toDomain() (*observabilitydomain.ErrorEvent, error) {
	tags := map[string]string{}
	if len(r.Tags) > 0 {
		// Stored tags are a JSON object; a non-string value is not a Sentry tag
		// and is dropped rather than failing the whole page.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(r.Tags, &raw); err != nil {
			return nil, fmt.Errorf("unmarshal event %s tags: %w", r.ID, err)
		}
		for key, value := range raw {
			var text string
			if json.Unmarshal(value, &text) == nil {
				tags[key] = text
			}
		}
	}
	return &observabilitydomain.ErrorEvent{
		ID: r.ID, EventID: r.EventID, ProjectID: r.ProjectID, IssueID: r.IssueID,
		Timestamp: r.Timestamp, Environment: r.Environment, Release: r.Release, Tags: tags,
	}, nil
}

func eventEvidenceRowsToDomain(rows []eventEvidenceRow) ([]*observabilitydomain.ErrorEvent, error) {
	out := make([]*observabilitydomain.ErrorEvent, 0, len(rows))
	for _, row := range rows {
		event, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, nil
}

func keysetCondition(after storecontracts.EventKeysetCursor) (string, []interface{}) {
	if after.ID == "" {
		return "", nil
	}
	return " AND (timestamp, id) < (?, ?::uuid)", []interface{}{after.Timestamp, after.ID}
}

// ListIssueEventEvidence pages one issue's events newest first. The workspace
// predicate is always applied, including for the administrative database role.
func (s *ErrorMonitoringStore) ListIssueEventEvidence(ctx context.Context, q storecontracts.IssueEventPageQuery) ([]*observabilitydomain.ErrorEvent, error) {
	if q.WorkspaceID == "" || q.IssueID == "" || q.Limit < 1 {
		return nil, fmt.Errorf("list issue event evidence: workspace, issue and limit are required")
	}
	condition, args := keysetCondition(q.After)
	query := `SELECT ` + eventEvidenceColumns + ` FROM ${SCHEMA_NAME}.error_events
		WHERE issue_id = ?::uuid AND workspace_id = ?::uuid` + condition + `
		ORDER BY timestamp DESC, id DESC LIMIT ?`
	params := append([]interface{}{q.IssueID, q.WorkspaceID}, args...)
	params = append(params, q.Limit)
	var rows []eventEvidenceRow
	if err := s.selectContext(ctx, &rows, query, params...); err != nil {
		return nil, TranslateSqlxError(err, "error_events")
	}
	return eventEvidenceRowsToDomain(rows)
}

// FindEventEvidenceByTag finds events carrying an exact tag key/value through
// JSONB containment (GIN-indexed), bounded by workspace, a lower time bound,
// optional environment/release, a page size and a statement timeout.
func (s *ErrorMonitoringStore) FindEventEvidenceByTag(ctx context.Context, q storecontracts.EventTagQuery) ([]*observabilitydomain.ErrorEvent, error) {
	if q.WorkspaceID == "" || q.TagKey == "" || q.TagValue == "" || q.Since.IsZero() || q.Limit < 1 {
		return nil, fmt.Errorf("find event evidence: workspace, tag, since and limit are required")
	}
	containment, err := json.Marshal(map[string]string{q.TagKey: q.TagValue})
	if err != nil {
		return nil, fmt.Errorf("find event evidence: %w", err)
	}
	query := `SELECT ` + eventEvidenceColumns + ` FROM ${SCHEMA_NAME}.error_events
		WHERE workspace_id = ?::uuid AND tags @> ?::jsonb AND timestamp >= ?`
	params := []interface{}{q.WorkspaceID, string(containment), q.Since}
	if q.Environment != "" {
		query += ` AND environment = ?`
		params = append(params, q.Environment)
	}
	if q.Release != "" {
		query += ` AND release = ?`
		params = append(params, q.Release)
	}
	condition, args := keysetCondition(q.After)
	query += condition + ` ORDER BY timestamp DESC, id DESC LIMIT ?`
	params = append(append(params, args...), q.Limit)

	var rows []eventEvidenceRow
	err = s.db.Transaction(ctx, func(tx context.Context) error {
		if _, err := s.execContext(tx, `SET LOCAL statement_timeout = '`+tagLookupStatementTimeout+`'`); err != nil {
			return err
		}
		return s.selectContext(tx, &rows, query, params...)
	})
	if err != nil {
		return nil, TranslateSqlxError(err, "error_events")
	}
	return eventEvidenceRowsToDomain(rows)
}
