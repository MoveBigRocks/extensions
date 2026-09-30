package sql

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	domain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	handlers "github.com/movebigrocks/extensions/error-tracking/runtime/handlers"
	services "github.com/movebigrocks/extensions/error-tracking/runtime/services"
	"github.com/stretchr/testify/require"
)

func TestIssueEvidenceUsesRealScopedStoreAndOmitsPrivateContext(t *testing.T) {
	store, ws, issueID := setupIssueCaseStore(t)
	service := services.NewIssueService(store, store, store, nil)
	_, err := store.execContext(t.Context(), `DELETE FROM ${SCHEMA_NAME}.issues WHERE id = ?`, issueID)
	require.NoError(t, err)
	seedEvent := domain.NewErrorEvent("44444444-4444-4444-8444-444444444444", "seed-event")
	issue := domain.NewIssue(seedEvent.ProjectID, "SDK failure", "handler.go", seedEvent)
	issue.WorkspaceID = ws
	require.NoError(t, store.CreateIssue(t.Context(), issue))
	issueID = issue.ID
	other := "22222222-2222-4222-8222-222222222222"
	for _, identity := range []string{ws, other, ""} {
		t.Run(identity, func(t *testing.T) {
			page, err := service.ReadIssuePage(t.Context(), identity, "", "", "", 1, 0)
			if identity == "" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if identity == ws {
				require.Len(t, page.Items, 1)
			} else {
				require.Empty(t, page.Items)
			}
		})
	}
	event := domain.NewErrorEvent(issue.ProjectID, "891b24ccf8a147219e873b6798e17bc6")
	event.IssueID = issueID
	event.Environment = "staging"
	event.Release = "release-123"
	event.User = &domain.UserContext{Email: "private-email"}
	event.Request = &domain.RequestContext{Headers: map[string]string{"Authorization": "private-token"}}
	event.Extra = domain.Metadata{"secret": "private-extra"}
	frames := make([]domain.FrameData, 60)
	for i := range frames {
		frames[i] = domain.FrameData{Filename: "handler.go", Function: "serve", LineNumber: i, Vars: domain.Metadata{"secret": "private-variable"}}
	}
	event.Exception = []domain.ExceptionData{{Type: "failure", Value: strings.Repeat("x", 2000), Stacktrace: &domain.StacktraceData{Frames: frames}}}
	require.NoError(t, store.CreateErrorEvent(t.Context(), event))
	evidence, err := service.ReadIssueEvidence(t.Context(), ws, issueID)
	require.NoError(t, err)
	require.Equal(t, "release-123", evidence.LatestEvent.Release)
	require.Len(t, evidence.LatestEvent.Exceptions[0].Frames, 50)
	require.Len(t, evidence.LatestEvent.Exceptions[0].Value, 1024)
	body, err := json.Marshal(evidence)
	require.NoError(t, err)
	require.NotContains(t, string(body), "private-")
	_, err = service.ReadIssueEvidence(t.Context(), other, issueID)
	require.Error(t, err)

	engine := gin.New()
	engine.Use(func(c *gin.Context) { c.Set("workspace_id", ws) })
	h := &handlers.IssueReadHandler{Service: service}
	engine.GET("/issues", h.List)
	engine.GET("/issues/:id", h.Get)
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/issues?workspace=" + ws, 200},
		{"/issues?workspace=" + other, 403},
		{"/issues?workspace=" + ws + "&limit=101", 400},
		{"/issues?workspace=" + ws + "&offset=-1", 400},
		{"/issues?workspace=" + ws + "&status=unexpected", 400},
		{"/issues/" + issueID + "?workspace=" + ws, 200},
		{"/issues/" + other + "?workspace=" + ws, 404},
	} {
		rr := httptest.NewRecorder()
		engine.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, tc.path, nil))
		require.Equal(t, tc.status, rr.Code, rr.Body.String())
		require.NotContains(t, rr.Body.String(), "private-")
	}
}
