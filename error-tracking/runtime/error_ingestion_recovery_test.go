package sql

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/movebigrocks/extension-sdk/extdb"
	domain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	services "github.com/movebigrocks/extensions/error-tracking/runtime/services"
	"github.com/stretchr/testify/require"
)

type recoveryPublisher func(context.Context, string, string, any) error

func (f recoveryPublisher) Publish(ctx context.Context, workspace, kind string, payload any) error {
	return f(ctx, workspace, kind, payload)
}

func recoveryIngestion(store *ErrorMonitoringStore, publish recoveryPublisher) *services.ErrorIngestion {
	return services.NewErrorIngestion(store, services.NewErrorGroupingService(store, store, publish), store.db)
}

func recoveryProject(t *testing.T, store *ErrorMonitoringStore, workspace, slug string) *domain.Project {
	t.Helper()
	project := domain.NewProject(workspace, "", "Recovery verification", slug, "go")
	require.NoError(t, store.CreateProject(context.Background(), "33333333-3333-4333-8333-333333333333", project))
	return project
}

func expirePending(t *testing.T, store *ErrorMonitoringStore) {
	t.Helper()
	_, err := store.execContext(context.Background(), `UPDATE ${SCHEMA_NAME}.error_events SET received = NOW() - INTERVAL '2 minutes', processed_at = NULL WHERE issue_id IS NULL`)
	require.NoError(t, err)
}

func TestErrorIngestionCommitsAcceptanceBeforePublicationAndRecoversFailure(t *testing.T) {
	store, workspace, _, _ := setupIssueCaseStoreDSN(t)
	store.db.SetMaxOpenConns(8)
	project := recoveryProject(t, store, workspace, "recovery")
	event := domain.NewErrorEvent(project.ID, "55555555555555555555555555555555")
	event.Message = "Recovery verification"
	failed := recoveryIngestion(store, func(_ context.Context, _ string, _ string, _ any) error {
		// A different connection sees acceptance before an external side effect.
		var count int
		require.NoError(t, store.getContext(context.Background(), &count, `SELECT count(*) FROM ${SCHEMA_NAME}.error_events WHERE event_id = ?`, event.EventID))
		require.Equal(t, 1, count)
		return errors.New("publication unavailable")
	})
	require.ErrorContains(t, failed.IngestEvent(context.Background(), event), "publication unavailable")
	expirePending(t, store)
	publications := 0
	restarted := recoveryIngestion(NewErrorMonitoringStore(store.db), func(ctx context.Context, ws, kind string, payload any) error {
		publications++
		require.Equal(t, workspace, ws)
		require.Equal(t, "issue.created", kind)
		body, err := json.Marshal(payload)
		require.NoError(t, err)
		return projectionHandler(store).HandleIssueCreated(context.Background(), body)
	})
	require.NoError(t, restarted.RecoverPendingEvents(context.Background()))
	require.Equal(t, 1, publications)
	persisted, err := store.GetErrorEvent(context.Background(), event.EventID)
	require.NoError(t, err)
	require.NotEmpty(t, persisted.IssueID)
	require.NotNil(t, persisted.GroupedAt)
	require.NoError(t, restarted.RecoverPendingEvents(context.Background()))
	require.Equal(t, 1, publications)
	require.NoError(t, restarted.IngestEvent(context.Background(), domain.NewErrorEvent(project.ID, event.EventID)))
	require.Equal(t, 1, publications, "projected replay does not republish")
	var count int
	require.NoError(t, store.getContext(context.Background(), &count, `SELECT event_count FROM ${SCHEMA_NAME}.projects WHERE id = ?`, project.ID))
	require.Equal(t, 1, count)
}

func TestErrorIngestionCommitFailureNeverPublishes(t *testing.T) {
	store, workspace, _ := setupIssueCaseStore(t)
	project := recoveryProject(t, store, workspace, "commit-failure")
	ctx := context.Background()
	_, err := store.execContext(ctx, `CREATE FUNCTION reject_acceptance_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected commit failure'; END $$`)
	require.NoError(t, err)
	_, err = store.execContext(ctx, `CREATE CONSTRAINT TRIGGER reject_acceptance_commit AFTER INSERT ON ${SCHEMA_NAME}.error_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_acceptance_commit()`)
	require.NoError(t, err)
	publications := 0
	ingestion := recoveryIngestion(store, func(context.Context, string, string, any) error { publications++; return nil })
	event := domain.NewErrorEvent(project.ID, "66666666666666666666666666666666")
	require.ErrorContains(t, ingestion.IngestEvent(ctx, event), "injected commit failure")
	require.Zero(t, publications)
	var count int
	require.NoError(t, store.getContext(ctx, &count, `SELECT count(*) FROM ${SCHEMA_NAME}.error_events WHERE project_id = ?`, project.ID))
	require.Zero(t, count)
}

func TestErrorRecoveryClaimsAreBoundedExclusiveAndExpireAfterWorkerLoss(t *testing.T) {
	store, workspace, _ := setupIssueCaseStore(t)
	store.db.SetMaxOpenConns(8)
	project := recoveryProject(t, store, workspace, "claims")
	for _, id := range []string{"77777777777777777777777777777777", "88888888888888888888888888888888"} {
		_, err := store.AcceptSentryEvent(context.Background(), domain.NewErrorEvent(project.ID, id))
		require.NoError(t, err)
	}
	expirePending(t, store)
	type claimed struct {
		events []*domain.ErrorEvent
		err    error
	}
	results := make(chan claimed, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			events, err := store.ClaimPendingErrorEvents(context.Background(), 1)
			results <- claimed{events, err}
		}()
	}
	wg.Wait()
	close(results)
	ids := map[string]bool{}
	for result := range results {
		require.NoError(t, result.err)
		require.Len(t, result.events, 1)
		ids[result.events[0].ID] = true
	}
	require.Len(t, ids, 2)
	pending, err := store.ClaimPendingErrorEvents(context.Background(), 100)
	require.NoError(t, err)
	require.Empty(t, pending)
	_, err = store.ClaimPendingErrorEvents(context.Background(), 101)
	require.Error(t, err)
	// Abandon both claims as if workers died. Expiry makes the same raw rows
	// available to a newly constructed store, without another incoming event.
	expirePending(t, store)
	pending, err = NewErrorMonitoringStore(store.db).ClaimPendingErrorEvents(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, pending, 2)
}

func TestErrorIngestionProcessCrashBeforePublication(t *testing.T) {
	if dsn := os.Getenv("MBR_TEST_CRASH_RECOVERY_DSN"); dsn != "" {
		db, err := extdb.Open(extdb.Config{DSN: dsn, MaxOpenConns: 4})
		require.NoError(t, err)
		store := NewErrorMonitoringStore(db)
		ingestion := recoveryIngestion(store, func(context.Context, string, string, any) error {
			// Terminate the actual child process after raw acceptance committed and
			// before host publication. No defers, shutdown hooks or in-memory queue survive.
			os.Exit(29)
			return nil
		})
		err = ingestion.IngestEvent(context.Background(), domain.NewErrorEvent(os.Getenv("MBR_TEST_CRASH_PROJECT"), "99999999999999999999999999999999"))
		t.Fatalf("expected process termination, got %v", err)
	}
	store, workspace, _, dsn := setupIssueCaseStoreDSN(t)
	store.db.SetMaxOpenConns(8)
	project := recoveryProject(t, store, workspace, "process-crash")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestErrorIngestionProcessCrashBeforePublication$")
	cmd.Env = append(os.Environ(), "MBR_TEST_CRASH_RECOVERY_DSN="+dsn, "MBR_TEST_CRASH_PROJECT="+project.ID)
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit, string(output))
	require.Equal(t, 29, exit.ExitCode(), string(output))
	expirePending(t, store)
	recovered := recoveryIngestion(store, func(_ context.Context, _ string, kind string, payload any) error {
		require.Equal(t, "issue.created", kind)
		body, err := json.Marshal(payload)
		require.NoError(t, err)
		return projectionHandler(store).HandleIssueCreated(context.Background(), body)
	})
	require.NoError(t, recovered.RecoverPendingEvents(context.Background()))
	event, err := store.GetErrorEvent(context.Background(), "99999999999999999999999999999999")
	require.NoError(t, err)
	require.NotEmpty(t, event.IssueID)
	issue, err := store.GetIssue(context.Background(), event.IssueID)
	require.NoError(t, err)
	require.EqualValues(t, 1, issue.EventCount)
}

func TestRecoveryContinuesPastFailedPublication(t *testing.T) {
	store, workspace, _ := setupIssueCaseStore(t)
	project := recoveryProject(t, store, workspace, "partial-recovery")
	for _, id := range []string{strings.Repeat("a", 32), strings.Repeat("b", 32)} {
		_, err := store.AcceptSentryEvent(context.Background(), domain.NewErrorEvent(project.ID, id))
		require.NoError(t, err)
	}
	expirePending(t, store)
	attempts := 0
	service := recoveryIngestion(store, func(context.Context, string, string, any) error { attempts++; return errors.New("host unavailable") })
	require.Error(t, service.RecoverPendingEvents(context.Background()))
	require.Equal(t, 2, attempts)
	require.NoError(t, service.RecoverPendingEvents(context.Background()))
	require.Equal(t, 2, attempts, "a failed batch waits for its lease before retrying")
}
