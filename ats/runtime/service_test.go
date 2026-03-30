package atsruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	atsdomain "github.com/movebigrocks/platform/extensions/ats/runtime/domain"
	"github.com/movebigrocks/platform/extensions/common/runtimehttp"
	automationdomain "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"
	platformsql "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
	"github.com/movebigrocks/platform/pkg/logger"
)

func TestATSServiceCreatesOwnedWorkflowAndStageAutomation(t *testing.T) {
	storeIface, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	store := storeIface.(*platformsql.Store)
	ctx := context.Background()
	require.NoError(t, ApplyMigrations(ctx, store.SqlxDB()))

	runtime, err := NewRuntime(store)
	require.NoError(t, err)
	t.Cleanup(func() {
		runtime.RulesEngine.Stop()
	})

	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	rule := automationdomain.NewRule(workspace.ID, "ATS Interview Follow-up", "admin")
	rule.IsActive = true
	rule.Conditions = automationdomain.RuleConditionsData{
		Operator: "and",
		Conditions: []automationdomain.RuleCondition{
			{Type: "event", Field: "trigger", Operator: "equals", Value: shareddomain.StringValue("ats_application_stage_changed")},
		},
	}
	rule.Actions = automationdomain.RuleActionsData{
		Actions: []automationdomain.RuleAction{
			{Type: "add_tags", Value: shareddomain.StringValue("ats-stage-review")},
		},
	}
	require.NoError(t, store.Rules().CreateRule(ctx, rule))

	job, err := runtime.Service.CreateJob(ctx, CreateJobInput{
		WorkspaceID:    workspace.ID,
		Slug:           "backend-engineer",
		Title:          "Backend Engineer",
		Team:           "Platform",
		Location:       "Amsterdam",
		WorkMode:       atsdomain.WorkModeHybrid,
		EmploymentType: atsdomain.EmploymentTypeFullTime,
		Summary:        "Own the API and data plane.",
		Description:    "Build and operate the recruiting runtime proof path.",
	})
	require.NoError(t, err)
	require.Equal(t, "backend-engineer-candidates", job.CaseQueueSlug)

	defaults, err := runtime.Service.WorkspaceDefaults(ctx, workspace.ID)
	require.NoError(t, err)
	require.Len(t, defaults.StagePresets, 3)
	require.Len(t, defaults.SavedFilters, 2)

	job, err = runtime.Service.PublishJob(ctx, workspace.ID, job.ID, job.CreatedAt)
	require.NoError(t, err)
	require.Equal(t, atsdomain.VacancyStatusOpen, job.Status)

	resumeAttachment := createUploadedAttachment(t, ctx, store, workspace.ID, "ada-lovelace-cv.pdf", []byte("%PDF-1.4 ada lovelace cv"))

	submission, err := runtime.Service.SubmitApplication(ctx, SubmitApplicationInput{
		WorkspaceID: workspace.ID,
		VacancySlug: job.Slug,
		Submission: atsdomain.CandidateSubmission{
			FullName:           "Ada Lovelace",
			Email:              "ada@example.com",
			Phone:              "+31 20 555 0100",
			Location:           "Amsterdam",
			LinkedInURL:        "https://linkedin.example/ada",
			PortfolioURL:       "https://portfolio.example/ada",
			CoverNote:          "I would like to help build the platform.",
			ResumeAttachmentID: resumeAttachment.ID,
			Source:             "careers_runtime_test",
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, submission.Application.CaseID)
	require.NotEmpty(t, submission.Applicant.ContactID)

	_, err = runtime.Service.AddRecruiterNote(ctx, workspace.ID, submission.Application.ID, "Strong profile, move to screening.", "Hiring Manager", "recruiter")
	require.NoError(t, err)

	application, err := runtime.Service.ChangeCandidateStage(ctx, workspace.ID, submission.Application.ID, StageChangeInput{
		Stage:     atsdomain.ApplicationStageScreening,
		ActorName: "Hiring Manager",
		Note:      "Screening call booked.",
	})
	require.NoError(t, err)
	require.Equal(t, atsdomain.ApplicationStageScreening, application.Stage)

	application, err = runtime.Service.ChangeCandidateStage(ctx, workspace.ID, submission.Application.ID, StageChangeInput{
		Stage:     atsdomain.ApplicationStageInterview,
		ActorName: "Hiring Manager",
		Note:      "Advance to panel interview.",
	})
	require.NoError(t, err)
	require.Equal(t, atsdomain.ApplicationStageInterview, application.Stage)

	candidateCase, err := store.Cases().GetCase(ctx, submission.Application.CaseID)
	require.NoError(t, err)
	require.Contains(t, candidateCase.Tags, "ats-stage-review")
	resumeAttachmentField, ok := candidateCase.CustomFields.GetString("ats_applicant_resume_attachment_id")
	require.True(t, ok)
	require.Equal(t, resumeAttachment.ID, resumeAttachmentField)
	portfolioField, ok := candidateCase.CustomFields.GetString("ats_applicant_portfolio_url")
	require.True(t, ok)
	require.Equal(t, "https://portfolio.example/ada", portfolioField)

	caseAttachments, err := store.Cases().ListCaseAttachments(ctx, workspace.ID, submission.Application.CaseID)
	require.NoError(t, err)
	require.Len(t, caseAttachments, 1)
	require.Equal(t, resumeAttachment.ID, caseAttachments[0].ID)
	require.Equal(t, submission.Application.CaseID, caseAttachments[0].CaseID)

	notes, err := runtime.ATSStore.ListRecruiterNotes(ctx, workspace.ID, submission.Application.ID)
	require.NoError(t, err)
	require.Len(t, notes, 3)

	job, err = runtime.Service.CloseJob(ctx, workspace.ID, job.ID, application.LastStageChangedAt)
	require.NoError(t, err)
	require.Equal(t, atsdomain.VacancyStatusClosed, job.Status)

	job, err = runtime.Service.ReopenJob(ctx, workspace.ID, job.ID, application.LastStageChangedAt)
	require.NoError(t, err)
	require.Equal(t, atsdomain.VacancyStatusOpen, job.Status)
}

func TestATSHandlerRunsLifecycleOverHTTP(t *testing.T) {
	storeIface, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	store := storeIface.(*platformsql.Store)
	ctx := context.Background()
	require.NoError(t, ApplyMigrations(ctx, store.SqlxDB()))

	runtime, err := NewRuntime(store)
	require.NoError(t, err)
	t.Cleanup(func() {
		runtime.RulesEngine.Stop()
	})

	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	engine := runtimehttp.DefaultEngine()
	RegisterRoutes(engine, runtime.Handler)
	server := httptest.NewServer(engine)
	defer server.Close()

	doJSON := func(method, path string, body any) map[string]any {
		t.Helper()
		var payload []byte
		if body != nil {
			var err error
			payload, err = json.Marshal(body)
			require.NoError(t, err)
		}
		req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(payload))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-MBR-Workspace-ID", workspace.ID)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		var decoded map[string]any
		if resp.StatusCode != http.StatusNoContent {
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))
		}
		require.Less(t, resp.StatusCode, 400, "unexpected response: %#v", decoded)
		return decoded
	}

	created := doJSON(http.MethodPost, "/extensions/ats/api/jobs", map[string]any{
		"slug":           "support-engineer",
		"title":          "Support Engineer",
		"team":           "Customer",
		"location":       "Remote",
		"workMode":       "remote",
		"employmentType": "full_time",
		"summary":        "Help customers win.",
		"description":    "Operate a high-signal inbox.",
	})
	jobID := created["id"].(string)

	doJSON(http.MethodPost, "/extensions/ats/api/jobs/"+jobID+"/publish", nil)
	resumeAttachment := createUploadedAttachment(t, ctx, store, workspace.ID, "grace-hopper-cv.pdf", []byte("%PDF-1.4 grace hopper cv"))

	submitted := doJSON(http.MethodPost, "/careers/applications", map[string]any{
		"vacancySlug":        "support-engineer",
		"fullName":           "Grace Hopper",
		"email":              "grace@example.com",
		"coverNote":          "I can help triage customer issues.",
		"portfolioUrl":       "https://portfolio.example/grace",
		"resumeAttachmentId": resumeAttachment.ID,
	})
	application := submitted["application"].(map[string]any)
	applicationID := application["id"].(string)
	caseID := application["caseId"].(string)

	doJSON(http.MethodPost, "/extensions/ats/api/applications/"+applicationID+"/notes", map[string]any{
		"body":       "Invite to recruiter screen.",
		"authorName": "Recruiter",
	})
	doJSON(http.MethodPost, "/extensions/ats/api/applications/"+applicationID+"/stage", map[string]any{
		"stage":     "screening",
		"actorName": "Recruiter",
	})

	listing := doJSON(http.MethodGet, "/extensions/ats/api/jobs/"+jobID+"/applications", nil)
	require.Len(t, listing["applications"].([]interface{}), 1)

	defaults := doJSON(http.MethodGet, "/extensions/ats/api/defaults", nil)
	require.Len(t, defaults["stagePresets"].([]interface{}), 3)

	caseAttachments, err := store.Cases().ListCaseAttachments(ctx, workspace.ID, caseID)
	require.NoError(t, err)
	require.Len(t, caseAttachments, 1)
	require.Equal(t, resumeAttachment.ID, caseAttachments[0].ID)
	require.Equal(t, caseID, caseAttachments[0].CaseID)

	doJSON(http.MethodPost, "/extensions/ats/api/jobs/"+jobID+"/close", nil)
	reopened := doJSON(http.MethodPost, "/extensions/ats/api/jobs/"+jobID+"/reopen", nil)
	require.Equal(t, string(atsdomain.VacancyStatusOpen), reopened["status"])
}

func TestATSServiceRejectsResumeAttachmentThatIsNotReady(t *testing.T) {
	storeIface, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	store := storeIface.(*platformsql.Store)
	ctx := context.Background()
	require.NoError(t, ApplyMigrations(ctx, store.SqlxDB()))

	runtime, err := NewRuntime(store)
	require.NoError(t, err)
	t.Cleanup(func() {
		runtime.RulesEngine.Stop()
	})

	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	job, err := runtime.Service.CreateJob(ctx, CreateJobInput{
		WorkspaceID:    workspace.ID,
		Slug:           "platform-engineer",
		Title:          "Platform Engineer",
		Team:           "Platform",
		Location:       "Amsterdam",
		WorkMode:       atsdomain.WorkModeHybrid,
		EmploymentType: atsdomain.EmploymentTypeFullTime,
		Summary:        "Build the platform.",
		Description:    "Own the durable product surface.",
	})
	require.NoError(t, err)
	_, err = runtime.Service.PublishJob(ctx, workspace.ID, job.ID, job.CreatedAt)
	require.NoError(t, err)

	pendingAttachment := servicedomain.NewAttachment(workspace.ID, "pending-resume.pdf", "application/pdf", int64(len([]byte("%PDF-1.4 pending"))), servicedomain.AttachmentSourceUpload)
	require.NoError(t, store.Cases().SaveAttachment(ctx, pendingAttachment, nil))

	_, err = runtime.Service.SubmitApplication(ctx, SubmitApplicationInput{
		WorkspaceID: workspace.ID,
		VacancySlug: job.Slug,
		Submission: atsdomain.CandidateSubmission{
			FullName:           "Ada Lovelace",
			Email:              "ada@example.com",
			CoverNote:          "Please review my application.",
			ResumeAttachmentID: pendingAttachment.ID,
			Source:             "careers_runtime_test",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not ready for ATS intake")
}

type testS3Server struct {
	server *httptest.Server
	mu     sync.Mutex
	puts   [][]byte
}

func createUploadedAttachment(t *testing.T, ctx context.Context, store *platformsql.Store, workspaceID, filename string, payload []byte) *servicedomain.Attachment {
	t.Helper()

	s3Server := newTestS3Server(t)
	attachmentService, err := serviceapp.NewAttachmentService(serviceapp.AttachmentServiceConfig{
		S3Endpoint:  s3Server.URL(),
		S3Region:    "us-east-1",
		S3Bucket:    "mbr-test-attachments",
		S3AccessKey: "test-access-key",
		S3SecretKey: "test-secret-key",
		Logger:      logger.NewNop(),
	})
	require.NoError(t, err)

	attachment := servicedomain.NewAttachment(workspaceID, filename, "application/pdf", int64(len(payload)), servicedomain.AttachmentSourceUpload)
	attachment.Description = "ATS resume proof attachment"
	require.NoError(t, attachmentService.Upload(ctx, attachment, bytes.NewReader(payload)))
	require.NoError(t, store.Cases().SaveAttachment(ctx, attachment, nil))
	return attachment
}

func newTestS3Server(t testing.TB) *testS3Server {
	t.Helper()

	s3 := &testS3Server{}
	s3.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s3.mu.Lock()
		s3.puts = append(s3.puts, body)
		s3.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(s3.server.Close)
	return s3
}

func (s *testS3Server) URL() string {
	return s.server.URL
}
