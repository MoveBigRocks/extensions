package sql

import (
	"context"
	"fmt"

	domain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	models "github.com/movebigrocks/extensions/error-tracking/sql-models"
)

// projectIssueEvent uses the persisted source event as the projection receipt.
// Its row lock covers both issue statistics and the link, so a failed link rolls
// back the statistics and a redelivery never applies the source event twice.
func (s *ErrorMonitoringStore) projectIssueEvent(ctx context.Context, workspaceID, projectID, eventID string, apply func(context.Context, *domain.ErrorEvent) (*domain.Issue, bool, error)) (*domain.Issue, bool, error) {
	if workspaceID == "" || projectID == "" || eventID == "" {
		return nil, false, fmt.Errorf("issue projection requires workspace, project and source event")
	}
	var result *domain.Issue
	var changed bool
	err := s.db.Transaction(ctx, func(tx context.Context) error {
		var row models.ErrorEvent
		if err := s.getContext(tx, &row, `SELECT * FROM ${SCHEMA_NAME}.error_events WHERE workspace_id = ? AND project_id = ? AND event_id = ? FOR UPDATE`, workspaceID, projectID, eventID); err != nil {
			return TranslateSqlxError(err, "error_events")
		}
		if row.IssueID != nil {
			var err error
			result, err = s.GetIssueInWorkspace(tx, workspaceID, *row.IssueID)
			return err
		}
		source, err := s.mapEventToDomain(&row)
		if err != nil {
			return err
		}
		result, changed, err = apply(tx, source)
		if err != nil {
			return err
		}
		if result.WorkspaceID != workspaceID || result.ProjectID != projectID {
			return fmt.Errorf("issue projection scope mismatch")
		}
		_, err = s.execContext(tx, `UPDATE ${SCHEMA_NAME}.error_events SET issue_id = ?, processed_at = COALESCE(processed_at, NOW()), grouped_at = NOW() WHERE id = ?`, result.ID, row.ID)
		return err
	})
	if err != nil {
		return nil, false, err
	}
	return result, changed, nil
}

func (s *ErrorMonitoringStore) ProjectIssueCreated(ctx context.Context, event domain.IssueCreatedEvent) (*domain.Issue, bool, error) {
	if event.Fingerprint == "" {
		return nil, false, fmt.Errorf("issue fingerprint is required")
	}
	return s.projectIssueEvent(ctx, event.WorkspaceID, event.ProjectID, event.FirstEventID, func(tx context.Context, source *domain.ErrorEvent) (*domain.Issue, bool, error) {
		// Issue row identity comes from PostgreSQL. The producer's proposed identity
		// cannot override an existing fingerprint or a persisted source-event link.
		issue := event.IssueForSource(source)
		return s.CreateOrUpdateIssueByFingerprint(tx, issue)
	})
}

func (s *ErrorMonitoringStore) ProjectIssueUpdated(ctx context.Context, event domain.IssueUpdatedEvent) (*domain.Issue, bool, error) {
	return s.projectIssueEvent(ctx, event.WorkspaceID, event.ProjectID, event.NewEventID, func(tx context.Context, source *domain.ErrorEvent) (*domain.Issue, bool, error) {
		issue, err := s.GetIssueInWorkspace(tx, event.WorkspaceID, event.IssueID)
		if err != nil {
			return nil, false, err
		}
		if issue.ProjectID != event.ProjectID {
			return nil, false, fmt.Errorf("issue projection project mismatch")
		}
		hasUser := source.HasIdentifiedUser()
		result, err := s.AtomicUpdateIssueStats(tx, event.WorkspaceID, issue.ID, source.EventID, source.Timestamp, hasUser)
		return result, err == nil, err
	})
}
