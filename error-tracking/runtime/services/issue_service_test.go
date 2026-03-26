package observabilityservices

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	errortrackingsql "github.com/movebigrocks/platform/extensions/error-tracking/runtime"
	obsdomain "github.com/movebigrocks/platform/extensions/error-tracking/runtime/domain"
	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
	platformsql "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	sharedomain "github.com/movebigrocks/platform/internal/shared/domain"
	testutil "github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/refext"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/id"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOutbox struct {
	mu     sync.Mutex
	events []mockPublishedEvent
}

type mockPublishedEvent struct {
	stream eventbus.Stream
	event  eventbus.Event
}

func (m *mockOutbox) Publish(context.Context, eventbus.Stream, interface{}) error {
	return nil
}

func (m *mockOutbox) PublishEvent(_ context.Context, stream eventbus.Stream, event eventbus.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, mockPublishedEvent{
		stream: stream,
		event:  event,
	})
	return nil
}

func (m *mockOutbox) getEvents() []mockPublishedEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]mockPublishedEvent, len(m.events))
	copy(out, m.events)
	return out
}

type failingOutbox struct {
	err        error
	mu         sync.Mutex
	publishCnt int
}

func (m *failingOutbox) Publish(context.Context, eventbus.Stream, interface{}) error {
	return m.err
}

func (m *failingOutbox) PublishEvent(_ context.Context, stream eventbus.Stream, event eventbus.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishCnt++
	return m.err
}

func (m *failingOutbox) publishedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.publishCnt
}

func newTestErrorStore(t testing.TB, store stores.Store) *errortrackingsql.ErrorMonitoringStore {
	t.Helper()

	concrete, ok := store.(*platformsql.Store)
	require.True(t, ok)

	rawDB, err := concrete.GetSQLDB()
	require.NoError(t, err)

	return errortrackingsql.NewErrorMonitoringStore(errortrackingsql.NewSqlxDB(rawDB, concrete.DB().Driver()))
}

func newTestProject(workspaceID string) *obsdomain.Project {
	projectID := id.New()
	project := obsdomain.NewProject(workspaceID, "", "Test Application "+projectID[:8], strings.ToLower(projectID[:12]), "javascript")
	project.ID = projectID
	project.CreatedAt = time.Now()
	project.UpdatedAt = time.Now()
	return project
}

func TestIssueService_ResolveIssuePublishesCasesBulkResolvedEvent(t *testing.T) {
	t.Parallel()

	store, cleanup := testutil.SetupTestStore(t)
	t.Cleanup(cleanup)

	baseCtx := context.Background()
	wsID := testutil.CreateTestWorkspace(t, store, "issue-service")
	refext.InstallAndActivateReferenceExtension(t, baseCtx, store, wsID, "error-tracking")
	errorStore := newTestErrorStore(t, store)

	project := newTestProject(wsID)
	require.NoError(t, errorStore.CreateProject(baseCtx, project))

	systemCase := testutil.NewIsolatedCase(t, wsID)
	systemCase.Source = sharedomain.SourceTypeSystem
	systemCase.IsSystemCase = true

	customerCase := testutil.NewIsolatedCase(t, wsID)
	customerCase.Source = sharedomain.SourceTypeManual

	require.NoError(t, store.Cases().CreateCase(baseCtx, systemCase))
	require.NoError(t, store.Cases().CreateCase(baseCtx, customerCase))

	issue := &obsdomain.Issue{
		ID:             "issue-for-bulk-resolve",
		WorkspaceID:    wsID,
		ProjectID:      project.ID,
		Title:          "Cannot connect to API",
		Fingerprint:    "fp-issue-1",
		Status:         obsdomain.IssueStatusUnresolved,
		FirstSeen:      time.Now(),
		LastSeen:       time.Now(),
		RelatedCaseIDs: []string{systemCase.ID, customerCase.ID},
		HasRelatedCase: true,
	}
	require.NoError(t, errorStore.CreateIssue(baseCtx, issue))

	outbox := &mockOutbox{}
	service := NewIssueService(
		errorStore,
		errorStore,
		errorStore,
		store.Workspaces(),
		outbox,
	)

	authCtx := graphshared.SetAuthContext(baseCtx, &platformdomain.AuthContext{
		Permissions: []string{platformdomain.PermissionIssueWrite},
	})
	err := service.ResolveIssue(authCtx, issue.ID, "", "")
	require.NoError(t, err)

	storedIssue, err := errorStore.GetIssue(baseCtx, issue.ID)
	require.NoError(t, err)
	assert.Equal(t, obsdomain.IssueStatusResolved, storedIssue.Status)
	assert.Equal(t, "fixed", storedIssue.Resolution)
	assert.NotNil(t, storedIssue.ResolvedAt)

	published := outbox.getEvents()
	require.Len(t, published, 2)

	var casesBulk *sharedomain.CasesBulkResolved
	var issueResolved *sharedomain.IssueResolved
	for _, e := range published {
		switch ev := e.event.(type) {
		case sharedomain.CasesBulkResolved:
			casesBulk = &ev
		case sharedomain.IssueResolved:
			issueResolved = &ev
		}
		assert.True(t,
			e.stream == eventbus.StreamIssueEvents || e.stream == eventbus.StreamCaseEvents,
			"unexpected event stream: %s", e.stream.String())
	}

	require.NotNil(t, issueResolved)
	assert.Equal(t, issue.ID, issueResolved.IssueID)
	assert.Equal(t, "fixed", issueResolved.Resolution)
	assert.NotZero(t, issueResolved.AffectedCaseCount)

	require.NotNil(t, casesBulk)
	assert.Equal(t, issue.ID, casesBulk.IssueID)
	assert.ElementsMatch(t, []string{systemCase.ID, customerCase.ID}, casesBulk.CaseIDs)
}

func TestIssueService_ResolveIssuePublishFailure_BestEffort(t *testing.T) {
	t.Parallel()

	store, cleanup := testutil.SetupTestSQLStore(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))
	refext.InstallAndActivateReferenceExtension(t, ctx, store, workspace.ID, "error-tracking")
	errorStore := newTestErrorStore(t, store)

	project := newTestProject(workspace.ID)
	require.NoError(t, errorStore.CreateProject(ctx, project))

	issueID := "issue-fail-transaction"
	issue := &obsdomain.Issue{
		ID:          issueID,
		WorkspaceID: workspace.ID,
		ProjectID:   project.ID,
		Title:       "Issue for publish failure",
		Fingerprint: "fp-tx-failure",
		Status:      obsdomain.IssueStatusUnresolved,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Platform:    "go",
	}
	require.NoError(t, errorStore.CreateIssue(ctx, issue))

	outbox := &failingOutbox{err: errors.New("outbox publish failed")}
	service := NewIssueService(
		errorStore,
		errorStore,
		errorStore,
		store.Workspaces(),
		outbox,
	)

	authCtx := graphshared.SetAuthContext(ctx, &platformdomain.AuthContext{
		Permissions: []string{platformdomain.PermissionIssueWrite},
	})
	// Without a transaction runner, publish failures are best-effort (logged, not returned)
	err := service.ResolveIssue(authCtx, issue.ID, "", "")
	require.NoError(t, err)
	require.Equal(t, 1, outbox.publishedCount())

	// Issue is still resolved even though publish failed
	updated, err := errorStore.GetIssue(ctx, issue.ID)
	require.NoError(t, err)
	assert.Equal(t, obsdomain.IssueStatusResolved, updated.Status)
	assert.NotNil(t, updated.ResolvedAt)
}
