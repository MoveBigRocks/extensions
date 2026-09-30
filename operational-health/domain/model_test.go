package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSourceCredentialAndSignalBoundaries(t *testing.T) {
	token := "oph_" + "abcdefghijklmnopqrstuvwxyz0123456789"
	hash := sha256.Sum256([]byte(token))
	now := time.Now().UTC()
	source := Source{ID: "0199a0c0-0000-7000-8000-00000000000a", Environment: "production", Enabled: true, TokenHashes: []string{hex.EncodeToString(hash[:])}, Components: map[string]Component{"api": {Checks: []string{"readiness", "process"}}}, Alerts: map[string]string{"APIDown": "api"}}
	cfg := Config{QueueID: source.ID, QueueSlug: "incidents", Sources: []Source{source}}
	require.NoError(t, cfg.Validate())
	_, err := cfg.Authenticate(source.ID, token)
	require.NoError(t, err)
	_, err = cfg.Authenticate(source.ID, token+"wrong")
	require.ErrorIs(t, err, ErrForbidden)
	cfg.Sources[0].Enabled = false
	_, err = cfg.Authenticate(source.ID, token)
	require.ErrorIs(t, err, ErrForbidden)
	batch := AlertBatch{Version: "4", Alerts: []Alert{{Status: "firing", Labels: map[string]string{"alertname": "APIDown", "severity": "critical", "environment": "production", "secret": "must not be retained"}, StartsAt: now.Add(-time.Minute), Fingerprint: "0123456789abcdef"}}}
	signals, _, err := Normalize(source, batch, now)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	encoded, err := json.Marshal(signals)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret")
	require.NotContains(t, string(encoded), "must not be retained")
	batch.Alerts[0].Labels["environment"] = "staging"
	_, _, err = Normalize(source, batch, now)
	require.ErrorIs(t, err, ErrForbidden)
	batch.Alerts[0].Labels["environment"] = "production"
	batch.Alerts[0].StartsAt = now.Add(time.Hour)
	_, _, err = Normalize(source, batch, now)
	require.ErrorIs(t, err, ErrInvalid)
	require.Equal(t, "resolved", Transition("resolved", "firing"))
}
func TestObservationCannotInventRecovery(t *testing.T) {
	now := time.Now()
	target := Component{Checks: []string{"readiness", "database"}}
	obs := Observation{ObservedAt: now, Checks: []Check{{Name: "readiness", State: "healthy"}}}
	require.Equal(t, "unknown", obs.State(target, now))
	obs.Checks = append(obs.Checks, Check{Name: "database", State: "healthy"})
	require.Equal(t, "healthy", obs.State(target, now))
	require.Equal(t, "unknown", obs.State(target, now.Add(181*time.Second)))
	obs.Checks[1].State = "unhealthy"
	require.Equal(t, "unhealthy", obs.State(target, now))
}
