package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/movebigrocks/extension-sdk/extdb"
	"github.com/movebigrocks/extension-sdk/runtimehost"
	"github.com/movebigrocks/extension-sdk/testdb"
	"github.com/movebigrocks/extensions/operational-health/domain"
	"github.com/movebigrocks/extensions/operational-health/migrations"
	"github.com/stretchr/testify/require"
)

const testWS = "0199a0c0-0000-7000-8000-000000000001"

func TestEvidenceRetriesRejectChangedPayloadWithoutPartialWrites(t *testing.T) {
	s, cfg, src := fixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	o := domain.Observation{Component: "api", ObservedAt: now, Checks: []domain.Check{{Name: "readiness", State: "healthy"}}}
	require.NoError(t, s.Observe(t.Context(), testWS, src, []domain.Observation{o}, now))
	require.NoError(t, s.Observe(t.Context(), testWS, src, []domain.Observation{o}, now.Add(time.Second)))
	changed := o
	changed.Release = "different"
	later := o
	later.ObservedAt = now.Add(time.Second)
	later.Checks = []domain.Check{{Name: "readiness", State: "unhealthy"}}
	require.ErrorIs(t, s.Observe(t.Context(), testWS, src, []domain.Observation{later, changed}, now), domain.ErrConflict)
	status, err := s.Status(t.Context(), testWS, cfg, now)
	require.NoError(t, err)
	require.NotNil(t, status[0].Observation)
	require.True(t, now.Equal(status[0].Observation.ObservedAt))
	require.Equal(t, "healthy", status[0].Observation.Checks[0].State)
	d := domain.Deployment{RunID: "run-1", Component: "api", StartsAt: now, ExpiresAt: now.Add(time.Minute), State: "started"}
	require.NoError(t, s.Deploy(t.Context(), testWS, src, d, now))
	require.NoError(t, s.Deploy(t.Context(), testWS, src, d, now.Add(time.Second)))
	d.ExpiresAt = now.Add(time.Hour)
	require.ErrorIs(t, s.Deploy(t.Context(), testWS, src, d, now), domain.ErrConflict)
}

func TestNewEpisodeLinksClosedIncidentWithoutReopeningIt(t *testing.T) {
	s, cfg, src := fixture(t)
	now := time.Now().UTC()
	sig := domain.Signal{Component: "api", AlertName: "APIDown", Severity: "critical", Fingerprint: "0123456789abcdef", StartsAt: now, Status: "firing"}
	_, err := s.Ingest(t.Context(), testWS, cfg, src, []domain.Signal{sig}, 0, false, now)
	require.NoError(t, err)
	page, err := s.List(t.Context(), testWS, domain.Filter{Limit: 10})
	require.NoError(t, err)
	first := page.Items[0].ID
	p, err := s.Claim(t.Context(), testWS, now.Add(time.Second))
	require.NoError(t, err)
	require.Equal(t, "create", p.Kind)
	caseID := "0199a0c0-0000-7000-8000-000000000077"
	require.NoError(t, s.Complete(t.Context(), testWS, *p, caseID, now))
	require.NoError(t, s.SyncCase(t.Context(), testWS, first, runtimehost.HostCase{ID: caseID, WorkspaceID: testWS, Status: "resolved"}, now))
	sig.StartsAt = now.Add(time.Minute)
	_, err = s.Ingest(t.Context(), testWS, cfg, src, []domain.Signal{sig}, 0, false, now.Add(time.Minute))
	require.NoError(t, err)
	page, err = s.List(t.Context(), testWS, domain.Filter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.NotEqual(t, first, page.Items[0].ID)
	require.NotNil(t, page.Items[1].ClosedAt)
	var payloads []string
	err = s.scoped(t.Context(), testWS, func(ctx context.Context) error {
		return s.DB.Get(ctx).SelectContext(ctx, &payloads, query(`SELECT payload::text FROM {s}.projection_outbox WHERE workspace_id=? AND operation_key=?`), testWS, "successor/"+page.Items[0].ID)
	})
	require.NoError(t, err)
	require.Len(t, payloads, 1)
	require.Contains(t, payloads[0], page.Items[0].ID)
}

func fixture(t *testing.T) (*Store, domain.Config, domain.Source) {
	t.Helper()
	dsn, cleanup := testdb.SetupPostgres(t)
	t.Cleanup(cleanup)
	db, err := extdb.Open(extdb.Config{DSN: dsn, MaxOpenConns: 12})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("CREATE SCHEMA " + Schema)
	require.NoError(t, err)
	migration, err := migrations.Files.ReadFile("000001_incident_hub.up.sql")
	require.NoError(t, err)
	_, err = db.Exec(strings.ReplaceAll(string(migration), "${SCHEMA_NAME}", Schema))
	require.NoError(t, err)
	hash := sha256.Sum256([]byte("oph_abcdefghijklmnopqrstuvwxyz0123456789"))
	src := domain.Source{ID: "0199a0c0-0000-7000-8000-000000000002", Environment: "production", Enabled: true, TokenHashes: []string{hex.EncodeToString(hash[:])}, Components: map[string]domain.Component{"api": {Checks: []string{"readiness"}}}, Alerts: map[string]string{"APIDown": "api"}}
	cfg := domain.Config{QueueID: "0199a0c0-0000-7000-8000-000000000003", QueueSlug: "incidents", Sources: []domain.Source{src}}
	return New(db), cfg, src
}
func TestConcurrentSignalsRecoveryAndProjectionFencing(t *testing.T) {
	s, cfg, src := fixture(t)
	ctx := t.Context()
	now := time.Now().UTC()
	sig := domain.Signal{Component: "api", AlertName: "APIDown", Severity: "critical", Fingerprint: "0123456789abcdef", StartsAt: now.Add(-time.Minute), Status: "firing"}
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			_, err := s.Ingest(ctx, testWS, cfg, src, []domain.Signal{sig}, 0, false, now)
			failures <- err
		})
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	page, err := s.List(ctx, testWS, domain.Filter{Limit: 20})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	incident := page.Items[0]
	require.Equal(t, "firing", incident.SignalState)
	p, err := s.Claim(ctx, testWS, now.Add(time.Second))
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Equal(t, "create", p.Kind)
	replacement, err := s.Claim(ctx, testWS, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.NotNil(t, replacement)
	require.NotEqual(t, p.Token, replacement.Token)
	require.ErrorIs(t, s.Complete(ctx, testWS, *p, "0199a0c0-0000-7000-8000-000000000010", now), domain.ErrConflict)
	require.NoError(t, s.Complete(ctx, testWS, *replacement, "0199a0c0-0000-7000-8000-000000000010", now))
	end := now
	sig.Status = "resolved"
	sig.EndsAt = &end
	_, err = s.Ingest(ctx, testWS, cfg, src, []domain.Signal{sig}, 0, false, now)
	require.NoError(t, err)
	sig.Status = "firing"
	sig.EndsAt = nil
	_, err = s.Ingest(ctx, testWS, cfg, src, []domain.Signal{sig}, 1, false, now)
	require.NoError(t, err)
	current, err := s.Get(ctx, testWS, incident.ID)
	require.NoError(t, err)
	require.Equal(t, "recovered", current.SignalState)
	require.Nil(t, current.ClosedAt)
	_, err = s.Get(ctx, "0199a0c0-0000-7000-8000-000000000099", incident.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}
func TestFreshnessCompletenessAndResolvedFirst(t *testing.T) {
	s, cfg, src := fixture(t)
	ctx := t.Context()
	now := time.Now().UTC()
	sig := domain.Signal{Component: "api", AlertName: "APIDown", Severity: "critical", Fingerprint: "0123456789abcdef", StartsAt: now.Add(-time.Minute), Status: "resolved", EndsAt: &now}
	_, err := s.Ingest(ctx, testWS, cfg, src, []domain.Signal{sig}, 0, true, now)
	require.NoError(t, err)
	page, err := s.List(ctx, testWS, domain.Filter{Limit: 20})
	require.NoError(t, err)
	require.Empty(t, page.Items)
	obs := domain.Observation{Component: "api", ObservedAt: now, Checks: []domain.Check{{Name: "readiness", State: "healthy"}}}
	require.NoError(t, s.Observe(ctx, testWS, src, []domain.Observation{obs}, now))
	states, err := s.Status(ctx, testWS, cfg, now)
	require.NoError(t, err)
	require.Equal(t, "healthy", states[0].State)
	old := obs
	old.ObservedAt = now.Add(-time.Hour)
	old.Checks = []domain.Check{{Name: "readiness", State: "unhealthy"}}
	require.NoError(t, s.Observe(ctx, testWS, src, []domain.Observation{old}, now))
	states, err = s.Status(ctx, testWS, cfg, now)
	require.NoError(t, err)
	require.Equal(t, "healthy", states[0].State)
	states, err = s.Status(ctx, testWS, cfg, now.Add(181*time.Second))
	require.NoError(t, err)
	require.Equal(t, "unknown", states[0].State)
}
func TestIncidentPaginationRejectsScopeChanges(t *testing.T) {
	s, cfg, src := fixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for _, component := range []string{"api", "web"} {
		src.Components[component] = domain.Component{Checks: []string{"readiness"}}
		_, err := s.Ingest(ctx, testWS, cfg, src, []domain.Signal{{Component: component, AlertName: "APIDown", Severity: "warning", Fingerprint: component, StartsAt: now, Status: "firing"}}, 0, false, now)
		require.NoError(t, err)
	}
	first, err := s.List(ctx, testWS, domain.Filter{Limit: 1})
	require.NoError(t, err)
	require.NotEmpty(t, first.Next)
	second, err := s.List(ctx, testWS, domain.Filter{Limit: 1, After: first.Next})
	require.NoError(t, err)
	require.Len(t, second.Items, 1)
	require.NotEqual(t, first.Items[0].ID, second.Items[0].ID)
	_, err = s.List(ctx, testWS, domain.Filter{Limit: 1, After: first.Next, Environment: "staging"})
	require.ErrorIs(t, err, domain.ErrInvalid)
}

func TestSnapshotDoesNotInventRecoveryAndRejectsOlderReplacement(t *testing.T) {
	s, cfg, src := fixture(t)
	ctx := t.Context()
	now := time.Now().UTC()
	sig := domain.Signal{Component: "api", AlertName: "APIDown", Severity: "critical", Fingerprint: "0123456789abcdef", StartsAt: now.Add(-time.Hour), Status: "firing"}
	_, err := s.Ingest(ctx, testWS, cfg, src, []domain.Signal{sig}, 0, false, now)
	require.NoError(t, err)
	require.NoError(t, s.Snapshot(ctx, testWS, cfg, src, nil, now.Add(time.Second), now.Add(time.Second)))
	page, err := s.List(ctx, testWS, domain.Filter{Limit: 20})
	require.NoError(t, err)
	require.Equal(t, "unknown", page.Items[0].SignalState)
	evidence, err := s.Evidence(ctx, testWS, page.Items[0].ID, domain.EvidenceFilter{Limit: 1})
	require.NoError(t, err)
	require.Equal(t, "unknown", evidence.Signals.Items[0].Status)
	require.Nil(t, evidence.Signals.Items[0].EndsAt)
	require.NotEmpty(t, evidence.Projections.Next)
	_, err = s.Evidence(ctx, testWS, page.Items[0].ID, domain.EvidenceFilter{Limit: 1, SignalsAfter: evidence.Projections.Next})
	require.ErrorIs(t, err, domain.ErrInvalid)
	require.NoError(t, s.Snapshot(ctx, testWS, cfg, src, []domain.Signal{sig}, now, now.Add(2*time.Second)))
	current, err := s.Get(ctx, testWS, page.Items[0].ID)
	require.NoError(t, err)
	require.Equal(t, "unknown", current.SignalState)
}
func TestFailedProbeStaysUnhealthyWhenDeliveryIsUnknown(t *testing.T) {
	s, cfg, src := fixture(t)
	now := time.Now().UTC()
	require.NoError(t, s.Observe(t.Context(), testWS, src, []domain.Observation{{Component: "api", ObservedAt: now, Checks: []domain.Check{{Name: "readiness", State: "unhealthy"}}}}, now))
	states, err := s.Status(t.Context(), testWS, cfg, now)
	require.NoError(t, err)
	require.Equal(t, "unhealthy", states[0].State)
	require.Equal(t, "unknown", states[0].DeliveryState)
}
func TestExpiredDeploymentRequiresAttentionEvenWithHealthyProbes(t *testing.T) {
	s, cfg, src := fixture(t)
	now := time.Now().UTC()
	require.NoError(t, s.Observe(t.Context(), testWS, src, []domain.Observation{{Component: "api", ObservedAt: now, Checks: []domain.Check{{Name: "readiness", State: "healthy"}}}}, now))
	require.NoError(t, s.Deploy(t.Context(), testWS, src, domain.Deployment{RunID: "run-1", Component: "api", StartsAt: now.Add(-20 * time.Minute), ExpiresAt: now.Add(-time.Minute), State: "started"}, now))
	states, err := s.Status(t.Context(), testWS, cfg, now)
	require.NoError(t, err)
	require.Equal(t, "expired", states[0].DeploymentState)
	require.Equal(t, "unhealthy", states[0].State)
	require.Nil(t, states[0].MaintenanceUntil)
}

func TestTenantPolicyAppliesToConfinedRuntimeRole(t *testing.T) {
	s, cfg, src := fixture(t)
	ctx := t.Context()
	now := time.Now().UTC()
	_, err := s.Ingest(ctx, testWS, cfg, src, []domain.Signal{{Component: "api", AlertName: "APIDown", Severity: "critical", Fingerprint: "0123456789abcdef", StartsAt: now, Status: "firing"}}, 0, false, now)
	require.NoError(t, err)
	role := fmt.Sprintf("oph_test_%d", time.Now().UnixNano())
	_, err = s.DB.Exec("CREATE ROLE " + role + " NOLOGIN NOSUPERUSER NOBYPASSRLS")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = s.DB.Exec("DROP OWNED BY " + role); _, _ = s.DB.Exec("DROP ROLE " + role) })
	_, err = s.DB.Exec("GRANT USAGE ON SCHEMA " + Schema + " TO " + role)
	require.NoError(t, err)
	_, err = s.DB.Exec("GRANT SELECT, INSERT ON ALL TABLES IN SCHEMA " + Schema + " TO " + role)
	require.NoError(t, err)
	// The core grants this permissive policy to confined extension runtimes.
	// Tenant isolation must remain restrictive when that policy is present.
	for _, table := range []string{"sources", "configuration_events", "deliveries", "incidents", "signals", "observations", "deployments", "projection_outbox"} {
		_, err = s.DB.Exec("CREATE POLICY mbr_runtime_access ON " + Schema + "." + table + " FOR ALL TO " + role + " USING (true) WITH CHECK (true)")
		require.NoError(t, err)
	}
	for _, workspace := range []string{testWS, "0199a0c0-0000-7000-8000-000000000099"} {
		require.NoError(t, s.DB.Transaction(ctx, func(tx context.Context) error {
			_, err := s.DB.Get(tx).ExecContext(tx, "SET LOCAL ROLE "+role)
			if err != nil {
				return err
			}
			_, err = s.DB.Get(tx).ExecContext(tx, `SELECT set_config('app.current_workspace_id',?,true)`, workspace)
			if err != nil {
				return err
			}
			var count int
			if err = s.DB.Get(tx).GetContext(tx, &count, "SELECT count(*) FROM "+Schema+".incidents"); err != nil {
				return err
			}
			expected := 0
			if workspace == testWS {
				expected = 1
			}
			require.Equal(t, expected, count)
			_, err = s.DB.Get(tx).ExecContext(tx, "SAVEPOINT forbidden_write")
			if err != nil {
				return err
			}
			_, err = s.DB.Get(tx).ExecContext(tx, "INSERT INTO "+Schema+".configuration_events (workspace_id,source_id,digest) VALUES (?::uuid,?::uuid,'forbidden')", "0199a0c0-0000-7000-8000-000000000098", src.ID)
			require.ErrorContains(t, err, "row-level security")
			_, err = s.DB.Get(tx).ExecContext(tx, "ROLLBACK TO SAVEPOINT forbidden_write")
			if err != nil {
				return err
			}
			return nil
		}))
	}
}
