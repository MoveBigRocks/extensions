package observabilityhandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/movebigrocks/extension-sdk/logger"
	"github.com/movebigrocks/extension-sdk/runtimehost"

	observabilitydomain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	observabilityservices "github.com/movebigrocks/extensions/error-tracking/runtime/services"
)

type issueCaseWriter interface {
	LinkIssueToCase(ctx context.Context, workspaceID, caseID, issueID, projectID string) error
	UnlinkIssueFromCase(ctx context.Context, workspaceID, caseID, issueID string) error
	CreateCaseForIssue(ctx context.Context, params observabilityservices.CreateCaseForIssueParams) (*runtimehost.HostCase, error)
	MarkCaseResolved(ctx context.Context, workspaceID, caseID string, resolvedAt time.Time) error
	MarkIssueResolved(ctx context.Context, workspaceID, caseID string, resolvedAt time.Time) error
}

type ErrorTrackingCaseEventHandler struct {
	caseService issueCaseWriter
	logger      *logger.Logger
}

func NewErrorTrackingCaseEventHandler(
	caseService issueCaseWriter,
	log *logger.Logger,
) *ErrorTrackingCaseEventHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &ErrorTrackingCaseEventHandler{
		caseService: caseService,
		logger:      log,
	}
}

func (h *ErrorTrackingCaseEventHandler) HandleIssueCaseLinked(ctx context.Context, eventData []byte) error {
	var event observabilitydomain.IssueCaseLinkedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal IssueCaseLinked event: %w", err)
	}
	if event.CaseID == "" || event.IssueID == "" {
		return fmt.Errorf("IssueCaseLinked event missing case_id or issue_id")
	}
	if event.LinkedBy == "" {
		return nil
	}

	return h.caseService.LinkIssueToCase(ctx, event.WorkspaceID, event.CaseID, event.IssueID, event.ProjectID)
}

func (h *ErrorTrackingCaseEventHandler) HandleIssueCaseUnlinked(ctx context.Context, eventData []byte) error {
	var event observabilitydomain.IssueCaseUnlinkedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal IssueCaseUnlinked event: %w", err)
	}
	if event.CaseID == "" || event.IssueID == "" {
		return fmt.Errorf("IssueCaseUnlinked event missing case_id or issue_id")
	}
	if event.UnlinkedBy == "" {
		return nil
	}

	return h.caseService.UnlinkIssueFromCase(ctx, event.WorkspaceID, event.CaseID, event.IssueID)
}

func (h *ErrorTrackingCaseEventHandler) HandleCaseCreatedForContact(ctx context.Context, eventData []byte) error {
	var event observabilitydomain.CaseCreatedForContactEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal CaseCreatedForContact event: %w", err)
	}
	if event.ContactEmail == "" || event.IssueID == "" {
		return nil
	}

	_, err := h.caseService.CreateCaseForIssue(ctx, observabilityservices.CreateCaseForIssueParams{
		WorkspaceID:  event.WorkspaceID,
		IssueID:      event.IssueID,
		ProjectID:    event.ProjectID,
		IssueTitle:   event.IssueTitle,
		IssueLevel:   event.IssueLevel,
		Priority:     event.Priority,
		ContactID:    event.ContactID,
		ContactEmail: event.ContactEmail,
	})
	return err
}

func (h *ErrorTrackingCaseEventHandler) HandleCasesBulkResolved(ctx context.Context, eventData []byte) error {
	var event observabilitydomain.CasesBulkResolvedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal CasesBulkResolved event: %w", err)
	}

	caseIDs := make([]string, 0, len(event.CaseIDs)+len(event.SystemCaseIDs)+len(event.CustomerCaseIDs))
	seen := make(map[string]struct{}, len(event.CaseIDs)+len(event.SystemCaseIDs)+len(event.CustomerCaseIDs))
	appendCaseID := func(caseID string) {
		if caseID == "" {
			return
		}
		if _, ok := seen[caseID]; ok {
			return
		}
		seen[caseID] = struct{}{}
		caseIDs = append(caseIDs, caseID)
	}
	for _, caseID := range event.CaseIDs {
		appendCaseID(caseID)
	}
	for _, caseID := range event.SystemCaseIDs {
		appendCaseID(caseID)
	}
	for _, caseID := range event.CustomerCaseIDs {
		appendCaseID(caseID)
	}
	if len(caseIDs) == 0 {
		return nil
	}

	for _, caseID := range caseIDs {
		if err := h.caseService.MarkCaseResolved(ctx, event.WorkspaceID, caseID, event.ResolvedAt); err != nil {
			h.logger.WithError(err).WithField("case_id", caseID).Warn("Failed to resolve case from error-tracking event")
			continue
		}
		if err := h.caseService.MarkIssueResolved(ctx, event.WorkspaceID, caseID, event.ResolvedAt); err != nil {
			h.logger.WithError(err).WithField("case_id", caseID).Warn("Failed to persist issue resolution on case")
		}
	}
	return nil
}
