package sql

import (
	"context"
	"sync"
	"testing"

	observabilitydomain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	"github.com/stretchr/testify/require"
)

func TestSentryAcceptanceConcurrentReplayRetainsOneScopedEventAndCount(t *testing.T) {
	store, workspace, _ := setupIssueCaseStore(t)
	store.db.SetMaxOpenConns(8)
	ctx := context.Background()
	projectID := "44444444-4444-4444-8444-444444444444"
	const eventID = "1234567890abcdef1234567890abcdef"
	type result struct {
		event   observabilitydomain.ErrorEvent
		created bool
		err     error
	}
	results := make(chan result, 6)
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			event := observabilitydomain.NewErrorEvent(projectID, eventID)
			event.Message = "Integration acceptance"
			created, err := store.AcceptSentryEvent(ctx, event)
			results <- result{*event, created, err}
		}()
	}
	wg.Wait()
	close(results)
	created := 0
	id := ""
	for result := range results {
		require.NoError(t, result.err)
		if result.created {
			created++
		}
		if id == "" {
			id = result.event.ID
		}
		require.Equal(t, id, result.event.ID)
	}
	require.Equal(t, 1, created)
	var eventCount int
	require.NoError(t, store.getContext(ctx, &eventCount, `SELECT event_count FROM ${SCHEMA_NAME}.projects WHERE id = ?`, projectID))
	require.Equal(t, 1, eventCount)
	replay := observabilitydomain.NewErrorEvent(projectID, eventID)
	replay.Message = "A replay cannot overwrite accepted evidence"
	inserted, err := store.AcceptSentryEvent(ctx, replay)
	require.NoError(t, err)
	require.False(t, inserted)
	require.Equal(t, "Integration acceptance", replay.Message)

	// Identical client IDs in different applications are different receipts.
	other := &observabilitydomain.Project{WorkspaceID: workspace, Name: "Other application", Slug: "other-application", Status: "active"}
	require.NoError(t, store.CreateProject(ctx, "33333333-3333-4333-8333-333333333333", other))
	different := observabilitydomain.NewErrorEvent(other.ID, eventID)
	inserted, err = store.AcceptSentryEvent(ctx, different)
	require.NoError(t, err)
	require.True(t, inserted)
	require.NotEqual(t, id, different.ID)
}

func TestSentryAcceptanceRollsBackEventWhenApplicationCountFails(t *testing.T) {
	store, _, _ := setupIssueCaseStore(t)
	ctx := context.Background()
	_, err := store.execContext(ctx, `CREATE FUNCTION reject_event_count() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected count write failure'; END $$`)
	require.NoError(t, err)
	_, err = store.execContext(ctx, `CREATE TRIGGER reject_event_count BEFORE UPDATE ON ${SCHEMA_NAME}.projects FOR EACH ROW EXECUTE FUNCTION reject_event_count()`)
	require.NoError(t, err)
	event := observabilitydomain.NewErrorEvent("44444444-4444-4444-8444-444444444444", "abcdef1234567890abcdef1234567890")
	_, err = store.AcceptSentryEvent(ctx, event)
	require.ErrorContains(t, err, "injected count write failure")
	var count int
	require.NoError(t, store.getContext(ctx, &count, `SELECT count(*) FROM ${SCHEMA_NAME}.error_events WHERE event_id = ?`, event.EventID))
	require.Zero(t, count)
	_, err = store.execContext(ctx, `DROP TRIGGER reject_event_count ON ${SCHEMA_NAME}.projects`)
	require.NoError(t, err)
	inserted, err := store.AcceptSentryEvent(ctx, event)
	require.NoError(t, err)
	require.True(t, inserted)
	var eventCount int
	require.NoError(t, store.getContext(ctx, &eventCount, `SELECT event_count FROM ${SCHEMA_NAME}.projects WHERE id = ?`, event.ProjectID))
	require.Equal(t, 1, eventCount)
}
