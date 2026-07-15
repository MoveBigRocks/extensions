package observabilityservices

import (
	"context"
	"testing"
	"time"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	"github.com/movebigrocks/extensions/error-tracking/runtime/hostclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type issueCaseFakeHost struct {
	created runtimehost.CreateCaseInput
	linked  runtimehost.LinkIssueToCaseInput
	cases   map[string]*runtimehost.HostCase
}

func newIssueCaseFakeHost() *issueCaseFakeHost {
	return &issueCaseFakeHost{cases: map[string]*runtimehost.HostCase{}}
}

func (f *issueCaseFakeHost) CreateCase(_ context.Context, input runtimehost.CreateCaseInput) (*runtimehost.HostCase, error) {
	f.created = input
	created := &runtimehost.HostCase{ID: "case-1", WorkspaceID: input.WorkspaceID, Subject: input.Subject, Description: input.Description, Priority: input.Priority, Channel: input.Channel, ContactID: input.ContactID, CustomFields: input.CustomFields}
	f.cases[created.ID] = created
	return created, nil
}
func (f *issueCaseFakeHost) GetCaseInWorkspace(_ context.Context, _, caseID string) (*runtimehost.HostCase, bool, error) {
	value, ok := f.cases[caseID]
	return value, ok, nil
}
func (f *issueCaseFakeHost) UpdateCase(_ context.Context, caseID string, patch runtimehost.CaseUpdateInput) (*runtimehost.HostCase, error) {
	value := f.cases[caseID]
	for key, item := range patch.CustomFields {
		value.CustomFields[key] = item
	}
	return value, nil
}
func (f *issueCaseFakeHost) MarkCaseResolvedInWorkspace(context.Context, string, string, time.Time) error {
	return nil
}
func (f *issueCaseFakeHost) LinkIssueToCase(_ context.Context, workspaceID, caseID, issueID, projectID string) error {
	f.linked = runtimehost.LinkIssueToCaseInput{WorkspaceID: workspaceID, IssueID: issueID, ProjectID: projectID}
	return nil
}
func (f *issueCaseFakeHost) UnlinkIssueFromCase(context.Context, string, string, string) error {
	return nil
}
func (f *issueCaseFakeHost) GetCaseByIssueAndContact(_ context.Context, _, _, _ string) (*runtimehost.HostCase, bool, error) {
	return nil, false, nil
}
func (f *issueCaseFakeHost) ListWorkspaces(context.Context) ([]runtimehost.HostWorkspace, error) {
	return nil, nil
}
func (f *issueCaseFakeHost) GetWorkspacesByIDs(context.Context, []string) ([]runtimehost.HostWorkspace, error) {
	return nil, nil
}
func (f *issueCaseFakeHost) PublishEvent(context.Context, runtimehost.PublishEventInput) error {
	return nil
}

func TestFormatIssueSubject(t *testing.T) {
	assert.Equal(t, "Error affecting you: NullPointerException", formatIssueSubject("NullPointerException"))
}

func TestFormatIssueDescription(t *testing.T) {
	assert.Equal(t, "We've detected an error that may be affecting your experience: Timeout", formatIssueDescription("Timeout", "warning"))
}

func TestIssueCaseServiceCreateCaseForIssueUsesHostAPI(t *testing.T) {
	fake := newIssueCaseFakeHost()
	service := NewIssueCaseService(func(context.Context) (hostclient.Host, error) { return fake, nil })

	created, err := service.CreateCaseForIssue(context.Background(), CreateCaseForIssueParams{
		WorkspaceID: "ws-1", IssueID: "issue-1", ProjectID: "project-1", IssueTitle: "Payment failed",
		IssueLevel: "error", Priority: "high", ContactID: "contact-1", ContactEmail: "a@example.com",
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "ws-1", fake.created.WorkspaceID)
	assert.Equal(t, "internal", fake.created.Channel)
	assert.Equal(t, "high", fake.created.Priority)
	assert.Equal(t, "issue-1", fake.created.CustomFields["linked_issue_id"])
	assert.Equal(t, "ws-1", fake.linked.WorkspaceID)
	assert.Equal(t, "issue-1", fake.linked.IssueID)
}
