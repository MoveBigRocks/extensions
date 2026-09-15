package observabilityservices

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/movebigrocks/extension-sdk/logger"
	"github.com/movebigrocks/extension-sdk/runtimehost"
	"github.com/movebigrocks/extensions/error-tracking/runtime/hostclient"
	"github.com/movebigrocks/extensions/error-tracking/runtime/storecontracts"
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

// ErrIssueCaseInFlight is returned when another attempt at the same issue and
// contact holds the claim and has not finished. The case-events consumer is
// at-least-once, so failing here lets the event be redelivered rather than
// creating a second case behind the attempt that is still running.
var ErrIssueCaseInFlight = errors.New("a case for this issue and contact is already being created")

// IssueCaseService owns the issue-to-case helper behavior for the error-tracking extension.
// It reaches core cases only through the language-neutral host API, and keeps
// the idempotency ledger for case creation in the extension's own schema.
type IssueCaseService struct {
	newHost hostclient.Provider
	claims  storecontracts.IssueCaseClaimStore
	logger  *logger.Logger
}

func NewIssueCaseService(newHost hostclient.Provider, claims storecontracts.IssueCaseClaimStore) *IssueCaseService {
	return &IssueCaseService{
		newHost: newHost,
		claims:  claims,
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
	if s == nil || s.newHost == nil || s.claims == nil {
		return nil, fmt.Errorf("issue case service is not configured")
	}
	if strings.TrimSpace(params.ContactEmail) == "" && strings.TrimSpace(params.ContactID) == "" {
		return nil, fmt.Errorf("a contact email or contact id is required to create a case for issue %s", params.IssueID)
	}
	host, err := s.newHost(ctx)
	if err != nil {
		return nil, err
	}

	// The workspace on the event is a claim, not a fact. Resolve the issue in
	// that workspace first and write only into the workspace it really lives in,
	// so a mislabelled or replayed event cannot put a customer's case, with the
	// error title in its subject, in front of another tenant.
	issue, err := s.claims.GetIssueInWorkspace(ctx, strings.TrimSpace(params.WorkspaceID), strings.TrimSpace(params.IssueID))
	if err != nil {
		return nil, fmt.Errorf("confirm issue %s in workspace %s: %w", params.IssueID, params.WorkspaceID, err)
	}
	workspaceID := issue.WorkspaceID
	dedupKey := issueCaseDedupKey(params)

	// A case created before this ledger existed has no claim, so keep the core
	// lookup as the first check; it is the only thing that dedups those.
	if params.ContactID != "" {
		existing, found, lookupErr := host.GetCaseByIssueAndContact(ctx, workspaceID, params.IssueID, params.ContactID)
		if lookupErr == nil && found {
			s.logger.WithFields(map[string]interface{}{
				"case_id":    existing.ID,
				"issue_id":   params.IssueID,
				"contact_id": params.ContactID,
			}).Debug("Returning existing case for issue/contact (idempotency)")
			return existing, nil
		}
	}

	// Take the claim before creating anything. Creating first and recording
	// afterwards is the shape that duplicates on redelivery: the second delivery
	// has nothing to find, so it creates a second case.
	claimed, err := s.claims.ClaimIssueCase(ctx, workspaceID, issue.ID, dedupKey)
	if err != nil {
		return nil, fmt.Errorf("claim case creation for issue %s: %w", issue.ID, err)
	}
	if !claimed {
		return s.caseFromExistingClaim(ctx, host, workspaceID, issue.ID, dedupKey)
	}

	caseObj, err := host.CreateCase(ctx, runtimehost.CreateCaseInput{
		WorkspaceID:  workspaceID,
		Subject:      formatIssueSubject(params.IssueTitle),
		Description:  formatIssueDescription(params.IssueTitle, params.IssueLevel),
		Priority:     params.Priority,
		Channel:      "internal",
		ContactID:    params.ContactID,
		ContactEmail: params.ContactEmail,
		CustomFields: map[string]any{
			"linked_issue_id":   issue.ID,
			"linked_project_id": params.ProjectID,
			"issue_level":       params.IssueLevel,
			"source":            "auto_monitoring",
			"auto_created":      true,
		},
	})
	if err != nil {
		// No case exists, so hand the claim back rather than making the next
		// delivery wait out the lease before it can try again.
		if releaseErr := s.claims.ReleaseIssueCaseClaim(ctx, workspaceID, issue.ID, dedupKey); releaseErr != nil {
			s.logger.WithError(releaseErr).WithField("issue_id", issue.ID).Warn("Failed to release the issue case claim after a failed create")
		}
		return nil, err
	}
	if err := s.claims.CompleteIssueCaseClaim(ctx, workspaceID, issue.ID, dedupKey, caseObj.ID); err != nil {
		return nil, fmt.Errorf("record case %s for issue %s: %w", caseObj.ID, issue.ID, err)
	}

	if err := host.LinkIssueToCase(ctx, workspaceID, caseObj.ID, issue.ID, params.ProjectID); err != nil {
		s.logger.Warn("Failed to link issue to case", "case_id", caseObj.ID, "issue_id", issue.ID, "error", err)
		return caseObj, nil
	}

	return caseObj, nil
}

// caseFromExistingClaim resolves a claim this caller did not take: either an
// earlier delivery already created the case, or one is still in flight.
func (s *IssueCaseService) caseFromExistingClaim(ctx context.Context, host hostclient.Host, workspaceID, issueID, dedupKey string) (*runtimehost.HostCase, error) {
	claim, err := s.claims.GetIssueCaseClaim(ctx, workspaceID, issueID, dedupKey)
	if err != nil {
		return nil, fmt.Errorf("load case claim for issue %s: %w", issueID, err)
	}
	if claim.CaseID == "" {
		return nil, ErrIssueCaseInFlight
	}
	existing, found, err := host.GetCaseInWorkspace(ctx, workspaceID, claim.CaseID)
	if err != nil {
		return nil, fmt.Errorf("load case %s for issue %s: %w", claim.CaseID, issueID, err)
	}
	if !found {
		return nil, fmt.Errorf("case %s recorded for issue %s no longer exists", claim.CaseID, issueID)
	}
	s.logger.WithFields(map[string]interface{}{
		"case_id":  existing.ID,
		"issue_id": issueID,
	}).Debug("Returning the case an earlier delivery created for this issue (idempotency)")
	return existing, nil
}

// issueCaseDedupKey is the identity a redelivered event has to fold onto: the
// contact the case is for. The email is preferred because the handler only
// forwards events that carry one, while the contact id is often empty on the
// same event — keying on the id would let an event with an email but no id skip
// deduplication entirely, which is how duplicate cases were created.
func issueCaseDedupKey(params CreateCaseForIssueParams) string {
	if email := strings.ToLower(strings.TrimSpace(params.ContactEmail)); email != "" {
		return "email:" + email
	}
	return "contact:" + strings.TrimSpace(params.ContactID)
}

func formatIssueSubject(issueTitle string) string {
	return fmt.Sprintf("Error affecting you: %s", strings.TrimSpace(issueTitle))
}

func formatIssueDescription(issueTitle, _ string) string {
	return fmt.Sprintf("We've detected an error that may be affecting your experience: %s", strings.TrimSpace(issueTitle))
}
