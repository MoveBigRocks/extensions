package observabilityservices

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/movebigrocks/extension-sdk/logger"
	"github.com/movebigrocks/extension-sdk/runtimehost"
	"github.com/movebigrocks/extensions/error-tracking/runtime/hostclient"
)

type CreateCaseForIssueParams struct {
	WorkspaceID  string
	IssueID      string
	ProjectID    string
	IssueTitle   string
	IssueLevel   string
	Priority     string
	ContactID    string
	ContactEmail string
}

// IssueCaseService owns the issue-to-case helper behavior for the error-tracking extension.
// It reaches core cases only through the language-neutral host API.
type IssueCaseService struct {
	newHost hostclient.Provider
	logger  *logger.Logger
}

func NewIssueCaseService(newHost hostclient.Provider) *IssueCaseService {
	return &IssueCaseService{
		newHost: newHost,
		logger:  logger.New().WithField("service", "issue-case"),
	}
}

func (s *IssueCaseService) LinkIssueToCase(ctx context.Context, workspaceID, caseID, issueID, projectID string) error {
	host, err := s.newHost(ctx)
	if err != nil {
		return err
	}
	return host.LinkIssueToCase(ctx, workspaceID, caseID, issueID, projectID)
}

func (s *IssueCaseService) UnlinkIssueFromCase(ctx context.Context, workspaceID, caseID, issueID string) error {
	host, err := s.newHost(ctx)
	if err != nil {
		return err
	}
	return host.UnlinkIssueFromCase(ctx, workspaceID, caseID, issueID)
}

func (s *IssueCaseService) MarkCaseResolved(ctx context.Context, workspaceID, caseID string, resolvedAt time.Time) error {
	host, err := s.newHost(ctx)
	if err != nil {
		return err
	}
	return host.MarkCaseResolvedInWorkspace(ctx, workspaceID, caseID, resolvedAt)
}

func (s *IssueCaseService) MarkIssueResolved(ctx context.Context, workspaceID, caseID string, resolvedAt time.Time) error {
	host, err := s.newHost(ctx)
	if err != nil {
		return err
	}
	_, err = host.UpdateCase(ctx, caseID, runtimehost.CaseUpdateInput{
		WorkspaceID: workspaceID,
		CustomFields: map[string]any{
			"issue_resolved":    true,
			"issue_resolved_at": resolvedAt.UTC().Format(time.RFC3339Nano),
		},
	})
	return err
}

func (s *IssueCaseService) CreateCaseForIssue(ctx context.Context, params CreateCaseForIssueParams) (*runtimehost.HostCase, error) {
	if s == nil || s.newHost == nil {
		return nil, fmt.Errorf("issue case service is not configured")
	}
	host, err := s.newHost(ctx)
	if err != nil {
		return nil, err
	}

	if params.ContactID != "" {
		existing, found, err := host.GetCaseByIssueAndContact(ctx, params.WorkspaceID, params.IssueID, params.ContactID)
		if err == nil && found {
			s.logger.WithFields(map[string]interface{}{
				"case_id":    existing.ID,
				"issue_id":   params.IssueID,
				"contact_id": params.ContactID,
			}).Debug("Returning existing case for issue/contact (idempotency)")
			return existing, nil
		}
	}

	caseObj, err := host.CreateCase(ctx, runtimehost.CreateCaseInput{
		WorkspaceID:  params.WorkspaceID,
		Subject:      formatIssueSubject(params.IssueTitle),
		Description:  formatIssueDescription(params.IssueTitle, params.IssueLevel),
		Priority:     params.Priority,
		Channel:      "internal",
		ContactID:    params.ContactID,
		ContactEmail: params.ContactEmail,
		CustomFields: map[string]any{
			"linked_issue_id":   params.IssueID,
			"linked_project_id": params.ProjectID,
			"issue_level":       params.IssueLevel,
			"source":            "auto_monitoring",
			"auto_created":      true,
		},
	})
	if err != nil {
		return nil, err
	}

	if err := host.LinkIssueToCase(ctx, params.WorkspaceID, caseObj.ID, params.IssueID, params.ProjectID); err != nil {
		s.logger.Warn("Failed to link issue to case", "case_id", caseObj.ID, "issue_id", params.IssueID, "error", err)
		return caseObj, nil
	}

	return caseObj, nil
}

func formatIssueSubject(issueTitle string) string {
	return fmt.Sprintf("Error affecting you: %s", strings.TrimSpace(issueTitle))
}

func formatIssueDescription(issueTitle, _ string) string {
	return fmt.Sprintf("We've detected an error that may be affecting your experience: %s", strings.TrimSpace(issueTitle))
}
