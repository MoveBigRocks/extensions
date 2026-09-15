package sql

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/extension-sdk/extdb"
	"github.com/movebigrocks/extension-sdk/testdb"
)

// setupIssueCaseStore brings the owned schema up on a bare database by running
// the canonical migration files, so the claim ledger is exercised against the
// SQL that ships in the bundle rather than a hand-written copy of it. The core
// tables the migrations reference are stubbed: they belong to the platform and
// are present in a real instance, but a test database has only this schema.
func setupIssueCaseStore(t *testing.T) (*ErrorMonitoringStore, string, string) {
	t.Helper()
	dsn, cleanup := testdb.SetupPostgres(t)
	t.Cleanup(cleanup)

	db, err := extdb.Open(extdb.Config{DSN: dsn})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	conn := db.Get(ctx)
	for _, statement := range []string{
		`CREATE SCHEMA IF NOT EXISTS core_platform`,
		`CREATE TABLE IF NOT EXISTS core_platform.workspaces (id UUID PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS core_platform.installed_extensions (id UUID PRIMARY KEY)`,
		`CREATE FUNCTION public.current_workspace_id() RETURNS UUID LANGUAGE sql STABLE AS $$ SELECT NULLIF(current_setting('mbr.workspace_id', TRUE), '')::uuid $$`,
		`CREATE SCHEMA IF NOT EXISTS ` + errorTrackingSchemaName,
	} {
		_, err = conn.ExecContext(ctx, statement)
		require.NoError(t, err)
	}

	paths, err := filepath.Glob(filepath.Join("..", "migrations", "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	sort.Strings(paths)
	for _, path := range paths {
		content, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		_, err = conn.ExecContext(ctx, strings.ReplaceAll(string(content), "${SCHEMA_NAME}", errorTrackingSchemaName))
		require.NoErrorf(t, err, "apply %s", path)
	}

	store := NewErrorMonitoringStore(db)
	workspaceID := "11111111-1111-4111-8111-111111111111"
	otherWorkspaceID := "22222222-2222-4222-8222-222222222222"
	installID := "33333333-3333-4333-8333-333333333333"
	projectID := "44444444-4444-4444-8444-444444444444"
	issueID := "55555555-5555-4555-8555-555555555555"
	for _, seed := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO core_platform.workspaces (id) VALUES ($1), ($2)`, []any{workspaceID, otherWorkspaceID}},
		{`INSERT INTO core_platform.installed_extensions (id) VALUES ($1)`, []any{installID}},
		{`INSERT INTO ` + errorTrackingSchemaName + `.projects (id, workspace_id, extension_install_id, name, slug) VALUES ($1, $2, $3, 'Checkout', 'checkout')`, []any{projectID, workspaceID, installID}},
		{`INSERT INTO ` + errorTrackingSchemaName + `.issues (id, workspace_id, extension_install_id, project_id, title, fingerprint) VALUES ($1, $2, $3, $4, 'Payment failed', 'fp-1')`, []any{issueID, workspaceID, installID, projectID}},
	} {
		_, err = conn.ExecContext(ctx, seed.query, seed.args...)
		require.NoError(t, err)
	}
	return store, workspaceID, issueID
}

func TestIssueCaseClaimIsTakenOnce(t *testing.T) {
	store, workspaceID, issueID := setupIssueCaseStore(t)
	ctx := context.Background()

	claimed, err := store.ClaimIssueCase(ctx, workspaceID, issueID, "email:a@example.com")
	require.NoError(t, err)
	require.True(t, claimed)

	// A redelivery of the same event finds the claim taken and does not create a
	// second case of its own.
	claimed, err = store.ClaimIssueCase(ctx, workspaceID, issueID, "email:a@example.com")
	require.NoError(t, err)
	require.False(t, claimed)

	claim, err := store.GetIssueCaseClaim(ctx, workspaceID, issueID, "email:a@example.com")
	require.NoError(t, err)
	require.Empty(t, claim.CaseID)

	caseID := "66666666-6666-4666-8666-666666666666"
	require.NoError(t, store.CompleteIssueCaseClaim(ctx, workspaceID, issueID, "email:a@example.com", caseID))
	claim, err = store.GetIssueCaseClaim(ctx, workspaceID, issueID, "email:a@example.com")
	require.NoError(t, err)
	require.Equal(t, caseID, claim.CaseID)

	// A completed claim is never taken over, and never released.
	claimed, err = store.ClaimIssueCase(ctx, workspaceID, issueID, "email:a@example.com")
	require.NoError(t, err)
	require.False(t, claimed)
	require.NoError(t, store.ReleaseIssueCaseClaim(ctx, workspaceID, issueID, "email:a@example.com"))
	claim, err = store.GetIssueCaseClaim(ctx, workspaceID, issueID, "email:a@example.com")
	require.NoError(t, err)
	require.Equal(t, caseID, claim.CaseID)

	// A different affected contact on the same issue claims separately.
	claimed, err = store.ClaimIssueCase(ctx, workspaceID, issueID, "email:b@example.com")
	require.NoError(t, err)
	require.True(t, claimed)
}

func TestIssueCaseClaimRejectsAnotherWorkspace(t *testing.T) {
	store, _, issueID := setupIssueCaseStore(t)
	ctx := context.Background()

	claimed, err := store.ClaimIssueCase(ctx, "22222222-2222-4222-8222-222222222222", issueID, "email:a@example.com")
	require.NoError(t, err)
	require.False(t, claimed)

	_, err = store.GetIssueCaseClaim(ctx, "22222222-2222-4222-8222-222222222222", issueID, "email:a@example.com")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestReleasedIssueCaseClaimCanBeRetaken(t *testing.T) {
	store, workspaceID, issueID := setupIssueCaseStore(t)
	ctx := context.Background()

	claimed, err := store.ClaimIssueCase(ctx, workspaceID, issueID, "email:a@example.com")
	require.NoError(t, err)
	require.True(t, claimed)

	// Case creation failed, so the claim is handed back and the next delivery
	// retries immediately instead of waiting out the lease.
	require.NoError(t, store.ReleaseIssueCaseClaim(ctx, workspaceID, issueID, "email:a@example.com"))
	claimed, err = store.ClaimIssueCase(ctx, workspaceID, issueID, "email:a@example.com")
	require.NoError(t, err)
	require.True(t, claimed)
}

func TestStaleIssueCaseClaimIsTakenOver(t *testing.T) {
	store, workspaceID, issueID := setupIssueCaseStore(t)
	ctx := context.Background()

	claimed, err := store.ClaimIssueCase(ctx, workspaceID, issueID, "email:a@example.com")
	require.NoError(t, err)
	require.True(t, claimed)

	// The runtime holding the claim died before it recorded a case. Age the claim
	// past the lease so a later delivery can take it over rather than being
	// blocked on it forever.
	_, err = store.execContext(ctx,
		`UPDATE ${SCHEMA_NAME}.issue_case_claims SET claimed_at = NOW() - INTERVAL '1 day'
		 WHERE workspace_id = ? AND issue_id = ? AND dedup_key = ?`,
		workspaceID, issueID, "email:a@example.com")
	require.NoError(t, err)

	claimed, err = store.ClaimIssueCase(ctx, workspaceID, issueID, "email:a@example.com")
	require.NoError(t, err)
	require.True(t, claimed)
}
