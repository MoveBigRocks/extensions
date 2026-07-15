package observabilityservices

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	obsdomain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	storecontracts "github.com/movebigrocks/extensions/error-tracking/runtime/storecontracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type issueStoreFake struct {
	issue *obsdomain.Issue
}

func (f *issueStoreFake) CreateIssue(_ context.Context, issue *obsdomain.Issue) error {
	f.issue = issue
	return nil
}
func (f *issueStoreFake) CreateOrUpdateIssueByFingerprint(_ context.Context, issue *obsdomain.Issue) (*obsdomain.Issue, bool, error) {
	f.issue = issue
	return issue, true, nil
}
func (f *issueStoreFake) GetIssue(context.Context, string) (*obsdomain.Issue, error) {
	if f.issue == nil {
		return nil, errors.New("not found")
	}
	return f.issue, nil
}
func (f *issueStoreFake) GetIssueInWorkspace(_ context.Context, workspaceID, _ string) (*obsdomain.Issue, error) {
	if f.issue == nil || f.issue.WorkspaceID != workspaceID {
		return nil, errors.New("not found")
	}
	return f.issue, nil
}
func (f *issueStoreFake) GetIssuesByIDs(context.Context, []string) ([]*obsdomain.Issue, error) {
	if f.issue == nil {
		return nil, nil
	}
	return []*obsdomain.Issue{f.issue}, nil
}
func (f *issueStoreFake) GetIssueByFingerprint(context.Context, string, string) (*obsdomain.Issue, error) {
	return f.issue, nil
}
func (f *issueStoreFake) UpdateIssue(_ context.Context, issue *obsdomain.Issue) error {
	f.issue = issue
	return nil
}
func (f *issueStoreFake) ListProjectIssues(context.Context, string, storecontracts.IssueFilter) ([]*obsdomain.Issue, error) {
	return []*obsdomain.Issue{f.issue}, nil
}
func (f *issueStoreFake) ListIssues(context.Context, storecontracts.IssueFilters) ([]*obsdomain.Issue, int, error) {
	return []*obsdomain.Issue{f.issue}, 1, nil
}
func (f *issueStoreFake) ListAllIssues(context.Context, storecontracts.IssueFilters) ([]*obsdomain.Issue, int, error) {
	return []*obsdomain.Issue{f.issue}, 1, nil
}
func (f *issueStoreFake) AtomicUpdateIssueStats(context.Context, string, string, string, time.Time, bool) (*obsdomain.Issue, error) {
	return f.issue, nil
}

type publishedEvent struct {
	workspaceID string
	eventType   string
	payload     any
}

type recordingPublisher struct {
	mu     sync.Mutex
	err    error
	events []publishedEvent
}

func (p *recordingPublisher) Publish(_ context.Context, workspaceID, eventType string, payload any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, publishedEvent{workspaceID: workspaceID, eventType: eventType, payload: payload})
	return p.err
}

func (p *recordingPublisher) snapshot() []publishedEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]publishedEvent(nil), p.events...)
}

func unresolvedIssue() *obsdomain.Issue {
	return &obsdomain.Issue{
		ID:             "issue-1",
		WorkspaceID:    "workspace-1",
		ProjectID:      "project-1",
		Title:          "Cannot connect to API",
		Fingerprint:    "fingerprint-1",
		Status:         obsdomain.IssueStatusUnresolved,
		FirstSeen:      time.Now(),
		LastSeen:       time.Now(),
		RelatedCaseIDs: []string{"case-system", "case-customer"},
		HasRelatedCase: true,
	}
}

func TestIssueServiceResolveIssuePublishesHostEvents(t *testing.T) {
	store := &issueStoreFake{issue: unresolvedIssue()}
	publisher := &recordingPublisher{}
	service := NewIssueService(store, nil, nil, publisher)

	require.NoError(t, service.ResolveIssue(context.Background(), store.issue.ID, "", ""))
	assert.Equal(t, obsdomain.IssueStatusResolved, store.issue.Status)
	assert.Equal(t, "fixed", store.issue.Resolution)
	require.NotNil(t, store.issue.ResolvedAt)

	events := publisher.snapshot()
	require.Len(t, events, 2)
	assert.Equal(t, "workspace-1", events[0].workspaceID)
	assert.Equal(t, "issue.resolved", events[0].eventType)
	resolved, ok := events[0].payload.(obsdomain.IssueResolvedEvent)
	require.True(t, ok)
	assert.Equal(t, store.issue.ID, resolved.IssueID)
	assert.Equal(t, 2, resolved.AffectedCaseCount)

	assert.Equal(t, "cases.bulk_resolved", events[1].eventType)
	bulk, ok := events[1].payload.(obsdomain.CasesBulkResolvedEvent)
	require.True(t, ok)
	assert.ElementsMatch(t, store.issue.RelatedCaseIDs, bulk.CaseIDs)
}

func TestIssueServiceResolveIssuePublishFailureIsBestEffort(t *testing.T) {
	store := &issueStoreFake{issue: unresolvedIssue()}
	publisher := &recordingPublisher{err: errors.New("host unavailable")}
	service := NewIssueService(store, nil, nil, publisher)

	require.NoError(t, service.ResolveIssue(context.Background(), store.issue.ID, "", ""))
	assert.Equal(t, obsdomain.IssueStatusResolved, store.issue.Status)
	require.NotNil(t, store.issue.ResolvedAt)
	assert.Len(t, publisher.snapshot(), 2)
}
