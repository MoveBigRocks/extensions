package sql

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/movebigrocks/extension-sdk/logger"
	domain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	handlers "github.com/movebigrocks/extensions/error-tracking/runtime/handlers"
	services "github.com/movebigrocks/extensions/error-tracking/runtime/services"
	"github.com/stretchr/testify/require"
)

func projectionHandler(store *ErrorMonitoringStore) *handlers.IssueEventHandler {
	return handlers.NewIssueEventHandler(services.NewIssueService(store, store, store, nil), logger.NewNop())
}

func acceptedProjectionEvent(t *testing.T, store *ErrorMonitoringStore, eventID string) *domain.ErrorEvent {
	t.Helper()
	event := domain.NewErrorEvent("44444444-4444-4444-8444-444444444444", eventID)
	event.Message = "Recovery verification"
	_, err := store.AcceptSentryEvent(context.Background(), event)
	require.NoError(t, err)
	return event
}

func creationPayload(t *testing.T, workspace string, event *domain.ErrorEvent) []byte {
	t.Helper()
	payload, err := json.Marshal(domain.IssueCreatedEvent{
		ProjectID: event.ProjectID, WorkspaceID: workspace, FirstEventID: event.EventID,
		Title: event.Message, Level: "info", Fingerprint: "recovery-fingerprint", CreatedAt: event.Timestamp,
	})
	require.NoError(t, err)
	return payload
}

func TestIssueProjectionRepeatedDeliveryCountsSourceEventOnce(t *testing.T) {
	store, workspace, _ := setupIssueCaseStore(t)
	event := acceptedProjectionEvent(t, store, "11111111111111111111111111111111")
	handler := projectionHandler(store)
	payload := creationPayload(t, workspace, event)
	require.NoError(t, handler.HandleIssueCreated(context.Background(), payload))
	// Reconstruct the consumer after the first commit, as on process restart.
	require.NoError(t, projectionHandler(store).HandleIssueCreated(context.Background(), payload))
	var count int
	require.NoError(t, store.getContext(context.Background(), &count, `SELECT event_count FROM ${SCHEMA_NAME}.issues WHERE fingerprint = 'recovery-fingerprint'`))
	require.Equal(t, 1, count)
}

func TestIssueProjectionLinkFailureRollsBackIssueAndRetries(t *testing.T) {
	store, workspace, _ := setupIssueCaseStore(t)
	event := acceptedProjectionEvent(t, store, "22222222222222222222222222222222")
	ctx := context.Background()
	_, err := store.execContext(ctx, `CREATE FUNCTION reject_issue_link() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected link write failure'; END $$`)
	require.NoError(t, err)
	_, err = store.execContext(ctx, `CREATE TRIGGER reject_issue_link BEFORE UPDATE OF issue_id ON ${SCHEMA_NAME}.error_events FOR EACH ROW EXECUTE FUNCTION reject_issue_link()`)
	require.NoError(t, err)
	payload := creationPayload(t, workspace, event)
	require.ErrorContains(t, projectionHandler(store).HandleIssueCreated(ctx, payload), "injected link write failure")
	var count int
	require.NoError(t, store.getContext(ctx, &count, `SELECT count(*) FROM ${SCHEMA_NAME}.issues WHERE fingerprint = 'recovery-fingerprint'`))
	require.Zero(t, count, "issue and event link must roll back together")
	_, err = store.execContext(ctx, `DROP TRIGGER reject_issue_link ON ${SCHEMA_NAME}.error_events`)
	require.NoError(t, err)
	require.NoError(t, projectionHandler(store).HandleIssueCreated(ctx, payload))
}

func TestIssueUpdatedProjectionRepeatedDeliveryCountsSourceEventOnce(t *testing.T) {
	store, workspace, _ := setupIssueCaseStore(t)
	first := acceptedProjectionEvent(t, store, "44444444444444444444444444444444")
	require.NoError(t, projectionHandler(store).HandleIssueCreated(context.Background(), creationPayload(t, workspace, first)))
	issue, err := store.GetIssueByFingerprint(context.Background(), first.ProjectID, "recovery-fingerprint")
	require.NoError(t, err)
	issueID := issue.ID
	event := acceptedProjectionEvent(t, store, "33333333333333333333333333333333")
	payload, err := json.Marshal(domain.IssueUpdatedEvent{
		WorkspaceID: workspace, ProjectID: event.ProjectID, IssueID: issueID, NewEventID: event.EventID,
		LastSeen: time.Now(),
	})
	require.NoError(t, err)
	handler := projectionHandler(store)
	require.NoError(t, handler.HandleIssueUpdated(context.Background(), payload))
	require.NoError(t, projectionHandler(store).HandleIssueUpdated(context.Background(), payload))
	var count int
	require.NoError(t, store.getContext(context.Background(), &count, `SELECT event_count FROM ${SCHEMA_NAME}.issues WHERE id = ?`, issueID))
	require.Equal(t, 2, count, "fixture's original event plus one accepted source event")
}

func TestConcurrentIssueProjectionAndOutOfOrderReplayPreserveCountsAndLatestEvidence(t *testing.T) {
	store, workspace, _ := setupIssueCaseStore(t)
	store.db.SetMaxOpenConns(12)
	newer := acceptedProjectionEvent(t, store, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	payload := creationPayload(t, workspace, newer)
	failures := make(chan error, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			failures <- projectionHandler(store).HandleIssueCreated(context.Background(), payload)
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	issue, err := store.GetIssueByFingerprint(context.Background(), newer.ProjectID, "recovery-fingerprint")
	require.NoError(t, err)
	require.EqualValues(t, 1, issue.EventCount)
	older := domain.NewErrorEvent(newer.ProjectID, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	older.Timestamp = newer.Timestamp.Add(-time.Hour)
	older.User = &domain.UserContext{ID: "verification-user"}
	_, err = store.AcceptSentryEvent(context.Background(), older)
	require.NoError(t, err)
	body, err := json.Marshal(domain.IssueUpdatedEvent{WorkspaceID: workspace, ProjectID: older.ProjectID, IssueID: issue.ID, NewEventID: older.EventID, LastSeen: time.Now().Add(time.Hour)})
	require.NoError(t, err)
	for range 2 {
		require.NoError(t, projectionHandler(store).HandleIssueUpdated(context.Background(), body))
	}
	updated, err := store.GetIssue(context.Background(), issue.ID)
	require.NoError(t, err)
	require.EqualValues(t, 2, updated.EventCount)
	require.EqualValues(t, 1, updated.UserCount)
	require.WithinDuration(t, newer.Timestamp, updated.LastSeen, time.Microsecond)
	require.Equal(t, newer.EventID, updated.LastEventID, "payload dates cannot overwrite persisted event chronology")
}

func TestIssueProjectionDoesNotCrossWorkspaceOrProject(t *testing.T) {
	store, workspace, _ := setupIssueCaseStore(t)
	first := acceptedProjectionEvent(t, store, "cccccccccccccccccccccccccccccccc")
	original := creationPayload(t, workspace, first)
	var event domain.IssueCreatedEvent
	require.NoError(t, json.Unmarshal(original, &event))
	event.WorkspaceID = "22222222-2222-4222-8222-222222222222"
	_, _, err := store.ProjectIssueCreated(context.Background(), event)
	require.Error(t, err)
	other := recoveryProject(t, store, event.WorkspaceID, "other-scope")
	second := domain.NewErrorEvent(other.ID, first.EventID)
	_, err = store.AcceptSentryEvent(context.Background(), second)
	require.NoError(t, err)
	// Equal client event IDs remain separate across project/workspace boundaries.
	require.NoError(t, projectionHandler(store).HandleIssueCreated(context.Background(), original))
	require.NoError(t, projectionHandler(store).HandleIssueCreated(context.Background(), creationPayload(t, other.WorkspaceID, second)))
	var distinct int
	require.NoError(t, store.getContext(context.Background(), &distinct, `SELECT count(DISTINCT issue_id) FROM ${SCHEMA_NAME}.error_events WHERE event_id = ?`, first.EventID))
	require.Equal(t, 2, distinct)
	// An unlinked event cannot be projected onto another application's issue.
	issue, err := store.GetIssueByFingerprint(context.Background(), first.ProjectID, "recovery-fingerprint")
	require.NoError(t, err)
	third := domain.NewErrorEvent(other.ID, "dddddddddddddddddddddddddddddddd")
	_, err = store.AcceptSentryEvent(context.Background(), third)
	require.NoError(t, err)
	_, _, err = store.ProjectIssueUpdated(context.Background(), domain.IssueUpdatedEvent{WorkspaceID: other.WorkspaceID, ProjectID: other.ID, IssueID: issue.ID, NewEventID: third.EventID})
	require.Error(t, err)
}

func TestIssueUpdateLinkFailureRollsBackCounter(t *testing.T) {
	store, workspace, _ := setupIssueCaseStore(t)
	first := acceptedProjectionEvent(t, store, "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	require.NoError(t, projectionHandler(store).HandleIssueCreated(context.Background(), creationPayload(t, workspace, first)))
	issue, err := store.GetIssueByFingerprint(context.Background(), first.ProjectID, "recovery-fingerprint")
	require.NoError(t, err)
	second := acceptedProjectionEvent(t, store, "ffffffffffffffffffffffffffffffff")
	ctx := context.Background()
	_, err = store.execContext(ctx, `CREATE FUNCTION reject_update_link() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected update link failure'; END $$`)
	require.NoError(t, err)
	_, err = store.execContext(ctx, `CREATE TRIGGER reject_update_link BEFORE UPDATE OF issue_id ON ${SCHEMA_NAME}.error_events FOR EACH ROW EXECUTE FUNCTION reject_update_link()`)
	require.NoError(t, err)
	event := domain.IssueUpdatedEvent{WorkspaceID: workspace, ProjectID: first.ProjectID, IssueID: issue.ID, NewEventID: second.EventID}
	_, _, err = store.ProjectIssueUpdated(ctx, event)
	require.ErrorContains(t, err, "injected update link failure")
	persisted, err := store.GetIssue(ctx, issue.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, persisted.EventCount)
	_, err = store.execContext(ctx, `DROP TRIGGER reject_update_link ON ${SCHEMA_NAME}.error_events`)
	require.NoError(t, err)
	_, _, err = store.ProjectIssueUpdated(ctx, event)
	require.NoError(t, err)
	persisted, err = store.GetIssue(ctx, issue.ID)
	require.NoError(t, err)
	require.EqualValues(t, 2, persisted.EventCount)
}
