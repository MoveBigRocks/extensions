package observabilityhandlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/movebigrocks/extension-sdk/logger"
	observabilitydomain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	"github.com/movebigrocks/extensions/error-tracking/runtime/metrics"
	observabilityservices "github.com/movebigrocks/extensions/error-tracking/runtime/services"
)

// IssueEventHandler handles issue-related events (IssueCreated, IssueUpdated, IssueResolved)
type IssueEventHandler struct {
	issueService *observabilityservices.IssueService
	logger       *logger.Logger
}

// NewIssueEventHandler creates a new issue event handler
func NewIssueEventHandler(issueService *observabilityservices.IssueService, log *logger.Logger) *IssueEventHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &IssueEventHandler{
		issueService: issueService,
		logger:       log,
	}
}

// HandleIssueCreated processes IssueCreated events
// This is where the actual storage write happens - decoupled from the grouping service!
// Uses atomic upsert to prevent race conditions when multiple workers process events
// with the same fingerprint concurrently.
func (h *IssueEventHandler) HandleIssueCreated(ctx context.Context, eventData []byte) error {
	// Parse the event
	var event observabilitydomain.IssueCreatedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal IssueCreated event: %w", err)
	}

	h.logger.WithFields(map[string]interface{}{
		"issue_id":     event.IssueID,
		"project_id":   event.ProjectID,
		"workspace_id": event.WorkspaceID,
	}).Info("Processing IssueCreated event")

	resultIssue, wasCreated, err := h.issueService.ProjectIssueCreated(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to project issue creation: %w", err)
	}
	actualIssueID := resultIssue.ID

	// 2. Update metrics (only for new issues)
	if wasCreated {
		severity := event.Level
		if severity == "" {
			severity = "error"
		}
		// Using CasesCreated as a proxy for issues created for now
		metrics.IssuesChanged.WithLabelValues(event.WorkspaceID, "created:"+severity).Inc()
	}

	// 3. Check alert rules (simplified for now)
	// In production, this would check alert conditions and trigger notifications
	h.logger.WithField("issue_id", actualIssueID).Debug("Checking alert rules")

	// 4. Log success
	h.logger.WithFields(map[string]interface{}{
		"issue_id":    actualIssueID,
		"project_id":  event.ProjectID,
		"title":       event.Title,
		"was_created": wasCreated,
	}).Info("Successfully processed IssueCreated event")

	return nil
}

// HandleIssueUpdated processes IssueUpdated events
// Uses atomic increment to prevent race conditions when multiple concurrent events
// update the same issue simultaneously.
func (h *IssueEventHandler) HandleIssueUpdated(ctx context.Context, eventData []byte) error {
	// Parse the event
	var event observabilitydomain.IssueUpdatedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal IssueUpdated event: %w", err)
	}

	h.logger.WithFields(map[string]interface{}{
		"issue_id":     event.IssueID,
		"project_id":   event.ProjectID,
		"has_new_user": event.HasNewUser,
	}).Info("Processing IssueUpdated event")

	issue, applied, err := h.issueService.ProjectIssueUpdated(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to project issue update: %w", err)
	}
	if !applied {
		return nil
	}

	// 3. Update metrics
	// Track error events using existing ErrorsIngested metric
	project, err := h.issueService.GetProject(ctx, issue.ProjectID)
	if err == nil && project != nil {
		metrics.ErrorsIngested.WithLabelValues(project.WorkspaceID, issue.ProjectID).Inc()
	}

	// 4. Log success with actual counts from database (not stale event values)
	h.logger.WithFields(map[string]interface{}{
		"issue_id":    event.IssueID,
		"event_count": issue.EventCount,
		"user_count":  issue.UserCount,
	}).Info("Successfully processed IssueUpdated event")

	return nil
}

// HandleIssueResolved processes IssueResolved events
func (h *IssueEventHandler) HandleIssueResolved(ctx context.Context, eventData []byte) error {
	// Parse the event
	var event observabilitydomain.IssueResolvedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal IssueResolved event: %w", err)
	}

	h.logger.WithFields(map[string]interface{}{
		"issue_id":   event.IssueID,
		"project_id": event.ProjectID,
		"resolution": event.Resolution,
	}).Info("Processing IssueResolved event")

	// 1. Get the issue
	issue, err := h.issueService.GetIssue(ctx, event.IssueID)
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}

	// 2. Update issue status
	issue.Status = observabilitydomain.IssueStatusResolved
	resolvedAt := event.ResolvedAt
	issue.ResolvedAt = &resolvedAt

	// 3. Save the updated issue
	if err := h.issueService.UpdateIssue(ctx, issue); err != nil {
		return fmt.Errorf("failed to update issue: %w", err)
	}

	// 4. Update metrics (using existing metrics as proxies)
	// Note: In production, we'd add specific IssuesResolved metric
	metrics.IssuesChanged.WithLabelValues(event.WorkspaceID, "resolved").Inc()

	// 5. Log success
	h.logger.WithFields(map[string]interface{}{
		"issue_id":       event.IssueID,
		"resolution":     event.Resolution,
		"affected_cases": event.AffectedCaseCount,
	}).Info("Successfully processed IssueResolved event")

	return nil
}
