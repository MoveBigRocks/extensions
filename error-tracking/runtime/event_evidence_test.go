package sql

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	domain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	handlers "github.com/movebigrocks/extensions/error-tracking/runtime/handlers"
	services "github.com/movebigrocks/extensions/error-tracking/runtime/services"
	"github.com/stretchr/testify/require"
)

const (
	evidenceOtherWorkspace = "22222222-2222-4222-8222-222222222222"
	evidenceInstall        = "33333333-3333-4333-8333-333333333333"
	evidenceProject        = "44444444-4444-4444-8444-444444444444"
	evidenceOtherProject   = "77777777-7777-4777-8777-777777777777"
	evidenceRequestID      = "019a9a3e-0f6b-7c3e-8f43-6c0b5a1e2d34"
)

func seedEvidenceEvent(t *testing.T, store *ErrorMonitoringStore, projectID, issueID, eventID, environment, release string, at time.Time, tags map[string]string) {
	t.Helper()
	event := domain.NewErrorEvent(projectID, eventID)
	event.IssueID = issueID
	event.Environment = environment
	event.Release = release
	event.Timestamp = at
	event.Tags = tags
	event.User = &domain.UserContext{Email: "private-user@example.com"}
	require.NoError(t, store.CreateErrorEvent(t.Context(), event))
}

func TestEventEvidenceTagsPagesAndExactTagLookupStayScoped(t *testing.T) {
	store, ws, seededIssue := setupIssueCaseStore(t)
	service := services.NewIssueService(store, store, store, nil)
	ctx := t.Context()
	_, err := store.execContext(ctx, `DELETE FROM ${SCHEMA_NAME}.issues WHERE id = ?`, seededIssue)
	require.NoError(t, err)
	createIssue := func(workspaceID, projectID, title string) string {
		issue := domain.NewIssue(projectID, title, "handler.go", domain.NewErrorEvent(projectID, "seed-"+title))
		issue.WorkspaceID = workspaceID
		require.NoError(t, store.CreateIssue(ctx, issue))
		return issue.ID
	}

	var indexes []string
	require.NoError(t, store.selectContext(ctx, &indexes, `SELECT indexname FROM pg_indexes WHERE schemaname = ? AND tablename = 'error_events' ORDER BY indexname`, errorTrackingSchemaName))
	require.Contains(t, indexes, "idx_error_events_tags")
	require.Contains(t, indexes, "idx_error_events_issue_timeline")
	require.Contains(t, indexes, "idx_error_events_issue_environment_release")

	// A second workspace with the same request id must never leak into lookups.
	_, err = store.execContext(ctx, `INSERT INTO ${SCHEMA_NAME}.projects (id, workspace_id, extension_install_id, name, slug) VALUES (?, ?, ?, 'Other', 'other')`, evidenceOtherProject, evidenceOtherWorkspace, evidenceInstall)
	require.NoError(t, err)
	issueID := createIssue(ws, evidenceProject, "SDK failure")
	otherIssue := createIssue(evidenceOtherWorkspace, evidenceOtherProject, "Other failure")

	now := time.Now().UTC().Truncate(time.Microsecond)
	tie := now.Add(-time.Hour)
	seedEvidenceEvent(t, store, evidenceProject, issueID, "a0000000000000000000000000000001", "staging", "web@1.2.3", tie, map[string]string{"request_id": "req-older-a", "route": "/graphql"})
	seedEvidenceEvent(t, store, evidenceProject, issueID, "a0000000000000000000000000000002", "staging", "web@1.2.3", tie, map[string]string{"request_id": "req-older-b", "route": "/graphql"})
	seedEvidenceEvent(t, store, evidenceProject, issueID, "a0000000000000000000000000000003", "production", "web@1.2.4", now.Add(-time.Minute), map[string]string{
		"request_id": evidenceRequestID, "route": "/graphql", "method": "POST", "status": "500", "user_id": "0199aa00-0000-7000-8000-000000000001",
		"user.email": "private@example.com", "url": "https://private.example/?token=private", "ip_address": "203.0.113.9",
		"note": "private@example.com", "Mixed-Case": "private-key",
	})
	// Outside the default seven-day lookup window.
	seedEvidenceEvent(t, store, evidenceProject, issueID, "a0000000000000000000000000000004", "production", "web@1.2.0", now.Add(-10*24*time.Hour), map[string]string{"request_id": evidenceRequestID})
	seedEvidenceEvent(t, store, evidenceOtherProject, otherIssue, "b0000000000000000000000000000001", "production", "web@1.2.4", now, map[string]string{"request_id": evidenceRequestID})

	t.Run("latest event evidence carries allowlisted tags only", func(t *testing.T) {
		evidence, err := service.ReadIssueEvidence(ctx, ws, issueID)
		require.NoError(t, err)
		require.Equal(t, map[string]string{"request_id": evidenceRequestID, "route": "/graphql", "method": "POST", "status": "500", "user_id": "0199aa00-0000-7000-8000-000000000001"}, evidence.LatestEvent.Tags)
		body, err := json.Marshal(evidence)
		require.NoError(t, err)
		require.NotContains(t, string(body), "private")
		require.NotContains(t, string(body), "203.0.113.9")
	})

	t.Run("issue events page newest first with a stable keyset across timestamp ties", func(t *testing.T) {
		first, err := service.ReadIssueEvents(ctx, ws, issueID, 2, "")
		require.NoError(t, err)
		require.Len(t, first.Items, 2)
		require.NotEmpty(t, first.Next)
		require.Equal(t, "a0000000000000000000000000000003", first.Items[0].EventID)
		require.Equal(t, issueID, first.Items[0].IssueID)
		require.Equal(t, evidenceProject, first.Items[0].ProjectID)
		require.Equal(t, "production", first.Items[0].Environment)
		require.Equal(t, "web@1.2.4", first.Items[0].Release)
		require.NotContains(t, first.Items[0].Tags, "user.email")
		second, err := service.ReadIssueEvents(ctx, ws, issueID, 2, first.Next)
		require.NoError(t, err)
		require.Len(t, second.Items, 2)
		require.Empty(t, second.Next)
		seen := map[string]bool{}
		for _, item := range append(first.Items, second.Items...) {
			require.False(t, seen[item.EventID], "duplicate across pages: %s", item.EventID)
			seen[item.EventID] = true
		}
		require.Len(t, seen, 4)
		require.Equal(t, "a0000000000000000000000000000004", second.Items[1].EventID)

		_, err = service.ReadIssueEvents(ctx, evidenceOtherWorkspace, issueID, 2, "")
		require.Error(t, err)
		require.NotErrorIs(t, err, services.ErrInvalidIssueRead)
		for _, bad := range []struct {
			limit int
			after string
		}{{0, ""}, {51, ""}, {2, "not a cursor"}, {2, "MTIzLm5vdC1hLXV1aWQ"}} {
			_, err = service.ReadIssueEvents(ctx, ws, issueID, bad.limit, bad.after)
			require.ErrorIs(t, err, services.ErrInvalidIssueRead)
		}
	})

	t.Run("exact tag lookup is workspace scoped, windowed and filterable", func(t *testing.T) {
		page, err := service.FindEventsByTag(ctx, ws, services.EventTagSearch{Key: "request_id", Value: evidenceRequestID, Limit: 10}, now)
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		require.Equal(t, "a0000000000000000000000000000003", page.Items[0].EventID)
		require.Equal(t, issueID, page.Items[0].IssueID)

		page, err = service.FindEventsByTag(ctx, ws, services.EventTagSearch{Key: "request_id", Value: evidenceRequestID, SinceHours: 24 * 30, Limit: 10}, now)
		require.NoError(t, err)
		require.Len(t, page.Items, 2)

		page, err = service.FindEventsByTag(ctx, ws, services.EventTagSearch{Key: "request_id", Value: evidenceRequestID, Release: "web@1.2.0", SinceHours: 24 * 30, Limit: 10}, now)
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		require.Equal(t, "a0000000000000000000000000000004", page.Items[0].EventID)

		page, err = service.FindEventsByTag(ctx, ws, services.EventTagSearch{Key: "route", Value: "/graphql", Environment: "staging", Limit: 1}, now)
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		require.NotEmpty(t, page.Next)
		next, err := service.FindEventsByTag(ctx, ws, services.EventTagSearch{Key: "route", Value: "/graphql", Environment: "staging", Limit: 1, After: page.Next}, now)
		require.NoError(t, err)
		require.Len(t, next.Items, 1)
		require.Empty(t, next.Next)
		require.NotEqual(t, page.Items[0].EventID, next.Items[0].EventID)

		other, err := service.FindEventsByTag(ctx, evidenceOtherWorkspace, services.EventTagSearch{Key: "request_id", Value: evidenceRequestID, Limit: 10}, now)
		require.NoError(t, err)
		require.Len(t, other.Items, 1)
		require.Equal(t, "b0000000000000000000000000000001", other.Items[0].EventID)

		for _, bad := range []services.EventTagSearch{
			{Key: "user.email", Value: "private@example.com", Limit: 10},
			{Key: "url", Value: "https://private.example/", Limit: 10},
			{Key: "Request_ID", Value: evidenceRequestID, Limit: 10},
			{Key: "request_id", Value: "someone@example.com", Limit: 10},
			{Key: "request_id", Value: "", Limit: 10},
			{Key: "request_id", Value: evidenceRequestID, Limit: 51},
			{Key: "request_id", Value: evidenceRequestID, Limit: 10, SinceHours: 24*90 + 1},
			{Key: "request_id", Value: evidenceRequestID, Limit: 10, Environment: "bad\nvalue"},
		} {
			_, err := service.FindEventsByTag(ctx, ws, bad, now)
			require.ErrorIs(t, err, services.ErrInvalidIssueRead, "%+v", bad)
		}
	})

	t.Run("issue list filters by environment and release of any event", func(t *testing.T) {
		for _, tc := range []struct {
			environment, release string
			want                 int
		}{{"staging", "", 1}, {"production", "web@1.2.4", 1}, {"", "web@1.2.0", 1}, {"development", "", 0}, {"staging", "web@1.2.4", 0}} {
			page, err := service.ReadIssuePage(ctx, ws, "", "", "", tc.environment, tc.release, 20, 0)
			require.NoError(t, err)
			require.Len(t, page.Items, tc.want, "%+v", tc)
		}
		page, err := service.ReadIssuePage(ctx, evidenceOtherWorkspace, "", "", "", "staging", "", 20, 0)
		require.NoError(t, err)
		require.Empty(t, page.Items)
	})

	t.Run("http reads", func(t *testing.T) {
		engine := gin.New()
		engine.Use(func(c *gin.Context) { c.Set("workspace_id", ws) })
		h := &handlers.IssueReadHandler{Service: service}
		engine.GET("/issues", h.List)
		engine.GET("/issues/:id/events", h.Events)
		engine.GET("/events", h.FindEvents)
		find := func(q url.Values) string { q.Set("workspace", ws); return "/events?" + q.Encode() }
		for _, tc := range []struct {
			path   string
			status int
		}{
			{"/issues/" + issueID + "/events?workspace=" + ws, 200},
			{"/issues/" + issueID + "/events?workspace=" + ws + "&limit=51", 400},
			{"/issues/" + issueID + "/events?workspace=" + ws + "&limit=x", 400},
			{"/issues/" + issueID + "/events?workspace=" + ws + "&after=%25", 400},
			{"/issues/" + issueID + "/events?workspace=" + evidenceOtherWorkspace, 403},
			{"/issues/" + otherIssue + "/events?workspace=" + ws, 404},
			{find(url.Values{"tag": {"request_id"}, "value": {evidenceRequestID}}), 200},
			{find(url.Values{"tag": {"request_id"}, "value": {evidenceRequestID}, "environment": {"production"}, "release": {"web@1.2.4"}, "sinceHours": {"24"}}), 200},
			{find(url.Values{"tag": {"user.email"}, "value": {"private@example.com"}}), 400},
			{find(url.Values{"tag": {"request_id"}}), 400},
			{find(url.Values{"tag": {"request_id"}, "value": {evidenceRequestID}, "sinceHours": {"x"}}), 400},
			{"/issues?workspace=" + ws + "&environment=staging&release=web%401.2.3", 200},
		} {
			rr := httptest.NewRecorder()
			engine.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, tc.path, nil))
			require.Equal(t, tc.status, rr.Code, "%s: %s", tc.path, rr.Body.String())
			require.NotContains(t, rr.Body.String(), "private")
			if tc.path[:7] == "/issues" && rr.Code == 200 && !strings.Contains(tc.path, "/events") {
				// The applied filters are echoed so clients can detect older runtimes.
				require.Contains(t, rr.Body.String(), `"filters":{"environment":"staging","release":"web@1.2.3"}`)
				require.Contains(t, rr.Body.String(), issueID)
			}
			if rr.Code == 200 && tc.path[:7] == "/events" {
				var body struct {
					Contract    string `json:"contract"`
					WorkspaceID string `json:"workspaceId"`
					Events      struct {
						Items []services.EventSummary `json:"items"`
					} `json:"events"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
				require.Equal(t, services.IssueReadContract, body.Contract)
				require.Equal(t, ws, body.WorkspaceID)
				require.Len(t, body.Events.Items, 1)
				require.Equal(t, evidenceRequestID, body.Events.Items[0].Tags["request_id"])
			}
		}
	})
}
