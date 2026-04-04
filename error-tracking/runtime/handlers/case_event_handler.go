package observabilityhandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	servicedomain "github.com/movebigrocks/extension-sdk/extensionhost/service/domain"
	"github.com/movebigrocks/extension-sdk/extensionhost/shared/contracts"
	shareddomain "github.com/movebigrocks/extension-sdk/extensionhost/shared/domain"
	"github.com/movebigrocks/extension-sdk/logger"

	observabilityservices "github.com/movebigrocks/extensions/error-tracking/runtime/services"
)

type issueCaseWriter interface {
	LinkIssueToCase(ctx context.Context, caseID, issueID, projectID string) error
	UnlinkIssueFromCase(ctx context.Context, caseID, issueID string) error
	CreateCaseForIssue(ctx context.Context, params observabilityservices.CreateCaseForIssueParams) (*servicedomain.Case, error)
	MarkCaseResolved(ctx context.Context, caseID string, resolvedAt time.Time) error
	GetCase(ctx context.Context, caseID string) (*servicedomain.Case, error)
	UpdateCase(ctx context.Context, caseObj *servicedomain.Case) error
}

type ErrorTrackingCaseEventHandler struct {
	caseService issueCaseWriter
	adminRunner contracts.AdminContextRunner
	logger      *logger.Logger
}

func NewErrorTrackingCaseEventHandler(
	caseService issueCaseWriter,
	adminRunner contracts.AdminContextRunner,
	log *logger.Logger,
) *ErrorTrackingCaseEventHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &ErrorTrackingCaseEventHandler{
		caseService: caseService,
		adminRunner: adminRunner,
		logger:      log,
	}
}

func (h *ErrorTrackingCaseEventHandler) HandleIssueCaseLinked(ctx context.Context, eventData []byte) error {
	var event shareddomain.IssueCaseLinked
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal IssueCaseLinked event: %w", err)
	}
	if event.CaseID == "" || event.IssueID == "" {
		return fmt.Errorf("IssueCaseLinked event missing case_id or issue_id")
	}
	if event.LinkedBy == "" {
		return nil
	}

	return h.adminRunner.WithAdminContext(ctx, func(adminCtx context.Context) error {
		return h.caseService.LinkIssueToCase(adminCtx, event.CaseID, event.IssueID, event.ProjectID)
	})
}

func (h *ErrorTrackingCaseEventHandler) HandleIssueCaseUnlinked(ctx context.Context, eventData []byte) error {
	var event shareddomain.IssueCaseUnlinked
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal IssueCaseUnlinked event: %w", err)
	}
	if event.CaseID == "" || event.IssueID == "" {
		return fmt.Errorf("IssueCaseUnlinked event missing case_id or issue_id")
	}
	if event.UnlinkedBy == "" {
		return nil
	}

	return h.adminRunner.WithAdminContext(ctx, func(adminCtx context.Context) error {
		return h.caseService.UnlinkIssueFromCase(adminCtx, event.CaseID, event.IssueID)
	})
}

func (h *ErrorTrackingCaseEventHandler) HandleCaseCreatedForContact(ctx context.Context, eventData []byte) error {
	var event shareddomain.CaseCreatedForContact
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal CaseCreatedForContact event: %w", err)
	}
	if event.ContactEmail == "" || event.IssueID == "" {
		return nil
	}

	return h.adminRunner.WithAdminContext(ctx, func(adminCtx context.Context) error {
		_, err := h.caseService.CreateCaseForIssue(adminCtx, observabilityservices.CreateCaseForIssueParams{
			WorkspaceID:  event.WorkspaceID,
			IssueID:      event.IssueID,
			ProjectID:    event.ProjectID,
			IssueTitle:   event.IssueTitle,
			IssueLevel:   event.IssueLevel,
			Priority:     servicedomain.CasePriority(event.Priority),
			ContactID:    event.ContactID,
			ContactEmail: event.ContactEmail,
		})
		return err
	})
}

func (h *ErrorTrackingCaseEventHandler) HandleCasesBulkResolved(ctx context.Context, eventData []byte) error {
	var event shareddomain.CasesBulkResolved
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

	return h.adminRunner.WithAdminContext(ctx, func(adminCtx context.Context) error {
		for _, caseID := range caseIDs {
			if err := h.caseService.MarkCaseResolved(adminCtx, caseID, event.ResolvedAt); err != nil {
				h.logger.WithError(err).WithField("case_id", caseID).Warn("Failed to resolve case from error-tracking event")
				continue
			}
			caseObj, err := h.caseService.GetCase(adminCtx, caseID)
			if err != nil {
				h.logger.WithError(err).WithField("case_id", caseID).Warn("Failed to reload case after resolution")
				continue
			}
			caseObj.MarkIssueResolved(event.ResolvedAt)
			if err := h.caseService.UpdateCase(adminCtx, caseObj); err != nil {
				h.logger.WithError(err).WithField("case_id", caseID).Warn("Failed to persist issue resolution on case")
			}
		}
		return nil
	})
}
