package observabilityservices

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	observabilitydomain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	"github.com/movebigrocks/extensions/error-tracking/runtime/hostclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// issueCaseFakeClaims stands in for the extension-owned claim ledger. It keeps
// the behaviour the SQL implementation relies on: an issue is only visible in
// its own workspace, and a claim is taken at most once until it is released.
type issueCaseFakeClaims struct {
	issues map[string]*observabilitydomain.Issue
	claims map[string]*observabilitydomain.IssueCaseClaim
}

func newIssueCaseFakeClaims(issues ...*observabilitydomain.Issue) *issueCaseFakeClaims {
	store := &issueCaseFakeClaims{
		issues: map[string]*observabilitydomain.Issue{},
		claims: map[string]*observabilitydomain.IssueCaseClaim{},
	}
	for _, issue := range issues {
		store.issues[issue.ID] = issue
	}
	return store
}

func (f *issueCaseFakeClaims) claimKey(workspaceID, issueID, dedupKey string) string {
	return strings.Join([]string{workspaceID, issueID, dedupKey}, "|")
}

func (f *issueCaseFakeClaims) GetIssueInWorkspace(_ context.Context, workspaceID, issueID string) (*observabilitydomain.Issue, error) {
	issue, ok := f.issues[issueID]
	if !ok || issue.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("not found")
	}
	return issue, nil
}

func (f *issueCaseFakeClaims) GetIssueCaseClaim(_ context.Context, workspaceID, issueID, dedupKey string) (*observabilitydomain.IssueCaseClaim, error) {
	claim, ok := f.claims[f.claimKey(workspaceID, issueID, dedupKey)]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return claim, nil
}

func (f *issueCaseFakeClaims) ClaimIssueCase(_ context.Context, workspaceID, issueID, dedupKey string) (bool, error) {
	key := f.claimKey(workspaceID, issueID, dedupKey)
	if _, ok := f.claims[key]; ok {
		return false, nil
	}
	f.claims[key] = &observabilitydomain.IssueCaseClaim{
		WorkspaceID: workspaceID,
		IssueID:     issueID,
		DedupKey:    dedupKey,
		ClaimedAt:   time.Now().UTC(),
	}
	return true, nil
}

func (f *issueCaseFakeClaims) CompleteIssueCaseClaim(_ context.Context, workspaceID, issueID, dedupKey, caseID string) error {
	claim, ok := f.claims[f.claimKey(workspaceID, issueID, dedupKey)]
	if !ok {
		return fmt.Errorf("not found")
	}
	claim.CaseID = caseID
	return nil
}

func (f *issueCaseFakeClaims) ReleaseIssueCaseClaim(_ context.Context, workspaceID, issueID, dedupKey string) error {
	key := f.claimKey(workspaceID, issueID, dedupKey)
	if claim, ok := f.claims[key]; ok && claim.CaseID == "" {
		delete(f.claims, key)
	}
	return nil
}

type issueCaseFakeHost struct {
	created      runtimehost.CreateCaseInput
	linked       runtimehost.LinkIssueToCaseInput
	cases        map[string]*runtimehost.HostCase
	createCalls  int
	createErr    error
	lookupByPair map[string]*runtimehost.HostCase
}

func newIssueCaseFakeHost() *issueCaseFakeHost {
	return &issueCaseFakeHost{cases: map[string]*runtimehost.HostCase{}, lookupByPair: map[string]*runtimehost.HostCase{}}
}

func (f *issueCaseFakeHost) CreateCase(_ context.Context, input runtimehost.CreateCaseInput) (*runtimehost.HostCase, error) {
	f.createCalls++
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = input
	created := &runtimehost.HostCase{ID: fmt.Sprintf("case-%d", f.createCalls), WorkspaceID: input.WorkspaceID, Subject: input.Subject, Description: input.Description, Priority: input.Priority, Channel: input.Channel, ContactID: input.ContactID, CustomFields: input.CustomFields}
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
func (f *issueCaseFakeHost) GetCaseByIssueAndContact(_ context.Context, workspaceID, issueID, contactID string) (*runtimehost.HostCase, bool, error) {
	found, ok := f.lookupByPair[strings.Join([]string{workspaceID, issueID, contactID}, "|")]
	return found, ok, nil
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

func issueCaseParams() CreateCaseForIssueParams {
	return CreateCaseForIssueParams{
		WorkspaceID: "ws-1", IssueID: "issue-1", ProjectID: "project-1", IssueTitle: "Payment failed",
		IssueLevel: "error", Priority: "high", ContactID: "contact-1", ContactEmail: "a@example.com",
	}
}

func newIssueCaseService(fake *issueCaseFakeHost, claims *issueCaseFakeClaims) *IssueCaseService {
	return NewIssueCaseService(func(context.Context) (hostclient.Host, error) { return fake, nil }, claims)
}

func TestIssueCaseServiceCreateCaseForIssueUsesHostAPI(t *testing.T) {
	fake := newIssueCaseFakeHost()
	claims := newIssueCaseFakeClaims(&observabilitydomain.Issue{ID: "issue-1", WorkspaceID: "ws-1"})
	service := newIssueCaseService(fake, claims)

	created, err := service.CreateCaseForIssue(context.Background(), issueCaseParams())
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "ws-1", fake.created.WorkspaceID)
	assert.Equal(t, "internal", fake.created.Channel)
	assert.Equal(t, "high", fake.created.Priority)
	assert.Equal(t, "issue-1", fake.created.CustomFields["linked_issue_id"])
	assert.Equal(t, "ws-1", fake.linked.WorkspaceID)
	assert.Equal(t, "issue-1", fake.linked.IssueID)
}

func TestIssueCaseServiceDedupsRedeliveredEventWithoutContactID(t *testing.T) {
	fake := newIssueCaseFakeHost()
	claims := newIssueCaseFakeClaims(&observabilitydomain.Issue{ID: "issue-1", WorkspaceID: "ws-1"})
	service := newIssueCaseService(fake, claims)
	ctx := context.Background()

	// An event that carries the affected user's email but no resolved contact id.
	// Deduplication used to be skipped entirely for these, so every redelivery of
	// the same event created another case for the same person.
	params := issueCaseParams()
	params.ContactID = ""

	first, err := service.CreateCaseForIssue(ctx, params)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := service.CreateCaseForIssue(ctx, params)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	assert.Equal(t, 1, fake.createCalls)

	// The same person on a later delivery that did resolve the contact id folds
	// onto the same claim, because the key is the email both deliveries carry.
	withContact, err := service.CreateCaseForIssue(ctx, issueCaseParams())
	require.NoError(t, err)
	assert.Equal(t, first.ID, withContact.ID)
	assert.Equal(t, 1, fake.createCalls)

	// A different affected user on the same issue still gets their own case.
	other := issueCaseParams()
	other.ContactID = ""
	other.ContactEmail = "b@example.com"
	otherCase, err := service.CreateCaseForIssue(ctx, other)
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, otherCase.ID)
	assert.Equal(t, 2, fake.createCalls)
}

func TestIssueCaseServiceReusesCaseCreatedBeforeTheClaimLedger(t *testing.T) {
	fake := newIssueCaseFakeHost()
	claims := newIssueCaseFakeClaims(&observabilitydomain.Issue{ID: "issue-1", WorkspaceID: "ws-1"})
	service := newIssueCaseService(fake, claims)

	// Cases created by an older version of this extension have no claim row, so
	// the core issue/contact lookup stays the first check for them.
	legacy := &runtimehost.HostCase{ID: "case-legacy", WorkspaceID: "ws-1"}
	fake.cases[legacy.ID] = legacy
	fake.lookupByPair["ws-1|issue-1|contact-1"] = legacy

	found, err := service.CreateCaseForIssue(context.Background(), issueCaseParams())
	require.NoError(t, err)
	assert.Equal(t, legacy.ID, found.ID)
	assert.Equal(t, 0, fake.createCalls)
}

func TestIssueCaseServiceRejectsIssueFromAnotherWorkspace(t *testing.T) {
	fake := newIssueCaseFakeHost()
	claims := newIssueCaseFakeClaims(&observabilitydomain.Issue{ID: "issue-1", WorkspaceID: "ws-1"})
	service := newIssueCaseService(fake, claims)

	// The event names a workspace the issue does not belong to. Writing the case
	// where the event said would put the error title, and the customer it names,
	// in another tenant's queue.
	params := issueCaseParams()
	params.WorkspaceID = "ws-2"

	_, err := service.CreateCaseForIssue(context.Background(), params)
	require.Error(t, err)
	assert.Equal(t, 0, fake.createCalls)
	assert.Empty(t, claims.claims)
}

func TestIssueCaseServiceReleasesClaimWhenCaseCreationFails(t *testing.T) {
	fake := newIssueCaseFakeHost()
	claims := newIssueCaseFakeClaims(&observabilitydomain.Issue{ID: "issue-1", WorkspaceID: "ws-1"})
	service := newIssueCaseService(fake, claims)
	ctx := context.Background()

	fake.createErr = fmt.Errorf("core unavailable")
	_, err := service.CreateCaseForIssue(ctx, issueCaseParams())
	require.Error(t, err)
	// Nothing was created, so the claim must not block the redelivery.
	assert.Empty(t, claims.claims)

	fake.createErr = nil
	created, err := service.CreateCaseForIssue(ctx, issueCaseParams())
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, 2, fake.createCalls)
	assert.Len(t, claims.claims, 1)
}

func TestIssueCaseServiceFailsWhileAnotherAttemptHoldsTheClaim(t *testing.T) {
	fake := newIssueCaseFakeHost()
	claims := newIssueCaseFakeClaims(&observabilitydomain.Issue{ID: "issue-1", WorkspaceID: "ws-1"})
	service := newIssueCaseService(fake, claims)

	params := issueCaseParams()
	params.ContactID = ""
	claimed, err := claims.ClaimIssueCase(context.Background(), "ws-1", "issue-1", issueCaseDedupKey(params))
	require.NoError(t, err)
	require.True(t, claimed)

	// A concurrent delivery must fail and be redelivered rather than race the
	// attempt that holds the claim into creating a second case.
	_, err = service.CreateCaseForIssue(context.Background(), params)
	require.ErrorIs(t, err, ErrIssueCaseInFlight)
	assert.Equal(t, 0, fake.createCalls)
}
