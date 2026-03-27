package observabilityhandlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	observabilitydomain "github.com/movebigrocks/platform/extensions/error-tracking/runtime/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

type stubSentryProjectStore struct {
	project            *observabilitydomain.Project
	getProjectErr      error
	gotProjectKey      string
	incrementCalls     int
	incrementWorkspace string
	incrementProject   string
	lastEventAt        *time.Time
}

func (s *stubSentryProjectStore) GetProjectByKey(_ context.Context, projectKey string) (*observabilitydomain.Project, error) {
	s.gotProjectKey = projectKey
	if s.getProjectErr != nil {
		return nil, s.getProjectErr
	}
	return s.project, nil
}

func (s *stubSentryProjectStore) IncrementEventCount(_ context.Context, workspaceID, projectID string, lastEventAt *time.Time) (int64, error) {
	s.incrementCalls++
	s.incrementWorkspace = workspaceID
	s.incrementProject = projectID
	s.lastEventAt = lastEventAt
	return 1, nil
}

type stubSentryEventStore struct {
	createCalls int
	lastEvent   *observabilitydomain.ErrorEvent
	createErr   error
}

func (s *stubSentryEventStore) CreateErrorEvent(_ context.Context, event *observabilitydomain.ErrorEvent) error {
	s.createCalls++
	s.lastEvent = event
	return s.createErr
}

type stubSentryProcessor struct {
	processCalls int
	lastEvent    *observabilitydomain.ErrorEvent
	processErr   error
}

func (s *stubSentryProcessor) ProcessEvent(_ context.Context, event *observabilitydomain.ErrorEvent) error {
	s.processCalls++
	s.lastEvent = event
	return s.processErr
}

func TestParseSentryEnvelope_SkipsNonEventItemsAndNormalizesEventID(t *testing.T) {
	envelope := strings.Join([]string{
		`{"event_id":"f4f4df0d-3a76-4a54-9657-fc4f8632c111","sent_at":"2026-03-27T12:00:00Z"}`,
		`{"type":"client_report"}`,
		`{"timestamp":"2026-03-27T12:00:01Z","discarded_events":[]}`,
		`{"type":"event"}`,
		`{"event_id":"f4f4df0d-3a76-4a54-9657-fc4f8632c111","message":"boom","level":"error"}`,
	}, "\n") + "\n"

	payload, err := parseSentryEnvelope([]byte(envelope))
	require.NoError(t, err)

	assert.Equal(t, "f4f4df0d3a764a549657fc4f8632c111", payload["event_id"])
	assert.Equal(t, "boom", payload["message"])
}

func TestConvertSentryEvent_SupportsSentryValuesContainers(t *testing.T) {
	raw := `{
		"event_id": "8c8cb1eec0bf4f8aa3f26e1323871887",
		"logentry": {"formatted": "formatted fallback"},
		"timestamp": 1710878412.25,
		"level": "error",
		"platform": "javascript",
		"logger": "sentry.javascript.node",
		"stacktrace": {
			"frames": [
				{"filename": "worker.js", "function": "runWorker", "lineno": 88, "colno": 13, "in_app": true}
			]
		},
		"exception": {
			"values": [
				{
					"type": "TypeError",
					"value": "Cannot read properties of undefined",
					"module": "worker",
					"stacktrace": {
						"frames": [
							{"filename": "app.js", "function": "handleCrash", "lineno": 42, "colno": 7, "in_app": true}
						]
					}
				}
			]
		},
		"breadcrumbs": {
			"values": [
				{
					"timestamp": 1710878400.5,
					"type": "http",
					"category": "xhr",
					"message": "GET /api/me",
					"level": "info",
					"data": {"method": "GET", "status_code": 200}
				}
			]
		},
		"user": {
			"id": "user-123",
			"email": "user@example.com",
			"username": "user",
			"ip_address": "127.0.0.1"
		},
		"tags": [["browser", "chrome"], ["runtime", "node"]],
		"extra": {"retry_count": 3},
		"contexts": {"runtime": "node"},
		"request": {
			"url": "https://app.example.com/dashboard",
			"method": "GET",
			"query_string": "tab=alerts",
			"headers": {"User-Agent": "sdk-test"},
			"cookies": {"session": "abc"},
			"data": {"mode": "debug"}
		}
	}`

	var data map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &data))

	event, err := convertSentryEvent(data, "project-1")
	require.NoError(t, err)

	assert.Equal(t, "formatted fallback", event.Message)
	assert.Equal(t, "error", event.Level)
	assert.Equal(t, "javascript", event.Platform)
	assert.Equal(t, "sentry.javascript.node", event.Logger)
	assert.Equal(t, time.Unix(1710878412, 250000000).UTC(), event.Timestamp)

	require.Len(t, event.Exception, 1)
	assert.Equal(t, "TypeError", event.Exception[0].Type)
	assert.Equal(t, "Cannot read properties of undefined", event.Exception[0].Value)
	assert.Equal(t, "worker", event.Exception[0].Module)
	require.NotNil(t, event.Exception[0].Stacktrace)
	require.Len(t, event.Exception[0].Stacktrace.Frames, 1)
	assert.Equal(t, "app.js", event.Exception[0].Stacktrace.Frames[0].Filename)

	require.NotNil(t, event.Stacktrace)
	require.Len(t, event.Stacktrace.Frames, 1)
	assert.Equal(t, "worker.js", event.Stacktrace.Frames[0].Filename)

	require.Len(t, event.Breadcrumbs, 1)
	assert.Equal(t, time.Unix(1710878400, 500000000).UTC(), event.Breadcrumbs[0].Timestamp)
	assert.Equal(t, "xhr", event.Breadcrumbs[0].Category)
	assert.Equal(t, "http", event.Breadcrumbs[0].Type)
	assert.Equal(t, "GET", event.Breadcrumbs[0].Data.GetString("method"))
	assert.Equal(t, int64(200), event.Breadcrumbs[0].Data.GetInt("status_code"))

	require.NotNil(t, event.User)
	assert.Equal(t, "user-123", event.User.ID)
	assert.Equal(t, "user@example.com", event.User.Email)
	assert.Equal(t, "127.0.0.1", event.User.IPAddr)

	assert.Equal(t, "chrome", event.Tags["browser"])
	assert.Equal(t, "node", event.Tags["runtime"])
	assert.Equal(t, int64(3), event.Extra.GetInt("retry_count"))
	assert.Equal(t, "node", event.Contexts.GetString("runtime"))

	require.NotNil(t, event.Request)
	assert.Equal(t, "https://app.example.com/dashboard", event.Request.URL)
	assert.Equal(t, "GET", event.Request.Method)
	assert.Equal(t, "tab=alerts", event.Request.QueryString)
	assert.Equal(t, "sdk-test", event.Request.Headers["User-Agent"])
	assert.Equal(t, "abc", event.Request.Cookies["session"])
	assert.Equal(t, "debug", event.Request.Data.GetString("mode"))
}

func TestHandleEnvelopeWithProject_AcceptsGzipAndValuesContainers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	projectStore := &stubSentryProjectStore{
		project: &observabilitydomain.Project{
			ID:            "project-1",
			WorkspaceID:   "workspace-1",
			ProjectNumber: 42,
			Status:        observabilitydomain.ProjectStatusActive,
		},
	}
	eventStore := &stubSentryEventStore{}
	processor := &stubSentryProcessor{}

	handler := NewSentryIngestHandler(projectStore, eventStore, processor, logger.NewNop())
	router := gin.New()
	router.POST("/api/:projectNumber/envelope", handler.HandleEnvelopeWithProject)

	envelope := strings.Join([]string{
		`{"event_id":"0f4f0fb7-e0ab-4a5d-80ec-3cb13f0b4e55","sent_at":"2026-03-27T12:00:00Z","sdk":{"name":"sentry.javascript.node","version":"7.120.0"}}`,
		`{"type":"event"}`,
		`{"event_id":"0f4f0fb7-e0ab-4a5d-80ec-3cb13f0b4e55","timestamp":1710878412.25,"level":"error","platform":"javascript","exception":{"values":[{"type":"TypeError","value":"Cannot read properties of undefined","stacktrace":{"frames":[{"filename":"app.js","function":"handleCrash","lineno":42,"in_app":true}]}}]},"breadcrumbs":{"values":[{"timestamp":1710878400.5,"type":"navigation","category":"navigation","message":"to /dashboard","data":{"from":"/login","to":"/dashboard"}}]}}`,
	}, "\n") + "\n"

	req := httptest.NewRequest(http.MethodPost, "/api/42/envelope", bytes.NewReader(gzipBytes(t, envelope)))
	req.Header.Set("Content-Type", "application/x-sentry-envelope")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("X-Sentry-Auth", "Sentry sentry_key=public-key-123,sentry_version=7")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "public-key-123", projectStore.gotProjectKey)
	assert.Equal(t, 1, eventStore.createCalls)
	assert.Equal(t, 1, processor.processCalls)
	assert.Equal(t, 1, projectStore.incrementCalls)
	assert.Equal(t, "workspace-1", projectStore.incrementWorkspace)
	assert.Equal(t, "project-1", projectStore.incrementProject)

	require.NotNil(t, eventStore.lastEvent)
	assert.Equal(t, "0f4f0fb7e0ab4a5d80ec3cb13f0b4e55", eventStore.lastEvent.EventID)
	require.Len(t, eventStore.lastEvent.Exception, 1)
	assert.Equal(t, "TypeError", eventStore.lastEvent.Exception[0].Type)
	require.Len(t, eventStore.lastEvent.Breadcrumbs, 1)
	assert.Equal(t, "navigation", eventStore.lastEvent.Breadcrumbs[0].Category)
	assert.Equal(t, "/dashboard", eventStore.lastEvent.Breadcrumbs[0].Data.GetString("to"))

	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, true, response["success"])
	assert.Equal(t, "0f4f0fb7e0ab4a5d80ec3cb13f0b4e55", response["event_id"])
}

func gzipBytes(t *testing.T, raw string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, err := writer.Write([]byte(raw))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return buffer.Bytes()
}
