package atsruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

	automationdomain "github.com/movebigrocks/extension-sdk/extensionhost/automation/domain"
	platformsql "github.com/movebigrocks/extension-sdk/extensionhost/infrastructure/stores/sql"
	platformdomain "github.com/movebigrocks/extension-sdk/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/extension-sdk/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/extension-sdk/extensionhost/service/services"
	shareddomain "github.com/movebigrocks/extension-sdk/extensionhost/shared/domain"
	"github.com/movebigrocks/extension-sdk/extensionhost/testutil"
	"github.com/movebigrocks/extension-sdk/logger"
	"github.com/movebigrocks/extension-sdk/runtimehttp"
	atsdomain "github.com/movebigrocks/extensions/ats/runtime/domain"
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
		Language:       "en",
		AboutTheJob:    "Build the product surface and the systems behind it.",
		Responsibilities: []string{
			"Own platform APIs",
			"Ship operator-facing product slices",
		},
		ResponsibilitiesHeading: "What you'll do",
		AboutYou:                "You care about product depth and reliable execution.",
		AboutYouHeading:         "Who you are",
		Profile: []string{
			"Strong backend engineering experience",
			"Comfort with product ambiguity",
		},
		OffersIntro: "You will work close to the product surface and operating model.",
		Offers: []string{
			"Small high-agency team",
			"Meaningful ownership",
		},
		OffersHeading: "Why join",
		Quote:         "Build useful things with calm people.",
	})
	require.NoError(t, err)
	require.NotEmpty(t, job.CaseQueueID)
	require.Equal(t, "backend-engineer-candidates", job.CaseQueueSlug)
	require.Equal(t, "en", job.PublicLanguage)
	require.Equal(t, "What you'll do", job.ResponsibilitiesHeading)
	require.Equal(t, []string{"Own platform APIs", "Ship operator-facing product slices"}, []string(job.Responsibilities))
	require.Equal(t, []string{"Small high-agency team", "Meaningful ownership"}, []string(job.Offers))

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
	require.Equal(t, atsdomain.ApplicationSourceKindATSPublic, submission.Application.SourceKind)

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
	require.Contains(t, candidateCase.Tags, "job:"+job.Slug)
	resumeAttachmentField, ok := candidateCase.CustomFields.GetString("ats_application_resume_attachment_id")
	require.True(t, ok)
	require.Equal(t, resumeAttachment.ID, resumeAttachmentField)
	portfolioField, ok := candidateCase.CustomFields.GetString("ats_applicant_portfolio_url")
	require.True(t, ok)
	require.Equal(t, "https://portfolio.example/ada", portfolioField)
	sourceKindField, ok := candidateCase.CustomFields.GetString("ats_application_source_kind")
	require.True(t, ok)
	require.Equal(t, string(atsdomain.ApplicationSourceKindATSPublic), sourceKindField)

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
		"language":       "nl",
		"aboutTheJob":    "Help shape the support operation and customer journey.",
		"responsibilities": []string{
			"Guide customers through issues",
			"Improve support workflows",
		},
		"responsibilitiesHeading": "De rol",
		"aboutYouHeading":         "Jouw profiel",
		"offersHeading":           "Wat bieden wij?",
		"quote":                   "Calm systems, fast help.",
	})
	jobID := created["id"].(string)
	require.Equal(t, "nl", created["language"])
	require.Equal(t, "De rol", created["responsibilitiesHeading"])
	require.Equal(t, "Calm systems, fast help.", created["quote"])

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
	require.Equal(t, "accepted", submitted["status"])
	require.Equal(t, "Support Engineer", submitted["job"].(map[string]any)["title"])
	_, hasApplicant := submitted["applicant"]
	require.False(t, hasApplicant)

	storedApplication, err := runtime.ATSStore.GetApplication(ctx, workspace.ID, applicationID)
	require.NoError(t, err)
	caseID := storedApplication.CaseID

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
	inbox := doJSON(http.MethodGet, "/extensions/ats/api/applications", nil)
	require.Len(t, inbox["applications"].([]interface{}), 1)

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

func TestATSCareersBundleAndUpdateEndpoints(t *testing.T) {
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
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))
		require.Less(t, resp.StatusCode, 400, "unexpected response: %#v", decoded)
		return decoded
	}

	bundle := doJSON(http.MethodGet, "/extensions/ats/api/careers", nil)
	require.Equal(t, "/careers", bundle["previewUrl"])
	require.NotEmpty(t, bundle["site"])
	require.Len(t, bundle["team"].([]interface{}), 3)
	require.Len(t, bundle["gallery"].([]interface{}), 4)
	require.NotEmpty(t, bundle["setup"])

	updatedSite := doJSON(http.MethodPut, "/extensions/ats/api/careers/site", map[string]any{
		"companyName":      "Move Big Rocks",
		"siteTitle":        "Careers at Move Big Rocks",
		"heroTitle":        "Build products with real leverage.",
		"primaryColor":     "#14532d",
		"backgroundColor":  "#f8faf7",
		"privacyPolicyUrl": "https://movebigrocks.com/privacy",
		"customCssEnabled": true,
		"customCss":        ".hero { outline: 1px solid transparent; }",
	})
	require.Equal(t, "Move Big Rocks", updatedSite["companyName"])
	require.Equal(t, "#14532d", updatedSite["primaryColor"])
	require.Equal(t, "https://movebigrocks.com/privacy", updatedSite["privacyPolicyUrl"])
	require.Equal(t, true, updatedSite["customCssEnabled"])

	team := doJSON(http.MethodPut, "/extensions/ats/api/careers/team", map[string]any{
		"team": []map[string]any{
			{
				"name":     "Ari Patel",
				"role":     "Product Engineer",
				"bio":      "Bridges product intent and backend execution.",
				"imageUrl": "",
			},
		},
	})
	require.Len(t, team["team"].([]interface{}), 1)

	gallery := doJSON(http.MethodPut, "/extensions/ats/api/careers/gallery", map[string]any{
		"gallery": []map[string]any{
			{
				"section":  "homepage",
				"altText":  "Design wall",
				"caption":  "Work in progress stays visible.",
				"imageUrl": "",
			},
		},
	})
	require.Len(t, gallery["gallery"].([]interface{}), 1)

	created := doJSON(http.MethodPost, "/extensions/ats/api/jobs", map[string]any{
		"slug":           "staff-engineer",
		"title":          "Staff Engineer",
		"team":           "Platform",
		"location":       "Amsterdam",
		"workMode":       "hybrid",
		"employmentType": "full_time",
		"summary":        "Own critical systems.",
		"description":    "Lead deep product and platform work.",
	})
	updatedJob := doJSON(http.MethodPut, "/extensions/ats/api/jobs/"+created["id"].(string), map[string]any{
		"title":                   "Principal Engineer",
		"team":                    "Platform",
		"location":                "Amsterdam",
		"workMode":                "hybrid",
		"employmentType":          "full_time",
		"language":                "en",
		"summary":                 "Own the hardest technical decisions.",
		"description":             "Lead architecture and delivery.",
		"aboutTheJob":             "You will shape the technical direction of the product.",
		"responsibilitiesHeading": "What you will own",
		"responsibilities":        []string{"System architecture", "Execution quality"},
		"aboutYouHeading":         "About you",
		"aboutYou":                "You combine technical depth with product judgment.",
		"profile":                 []string{"Strong backend engineering background"},
		"offersHeading":           "What we offer",
		"offersIntro":             "A role with leverage and trust.",
		"offers":                  []string{"Meaningful ownership"},
		"quote":                   "Make the whole system sharper.",
	})
	require.Equal(t, "Principal Engineer", updatedJob["title"])
	require.Equal(t, "/careers/jobs/staff-engineer", updatedJob["careersPath"])
}

func TestRenderCareersSiteProducesHomepageAndJobPage(t *testing.T) {
	bundle := &CareersSiteBundle{
		Site: CareersSiteProfile{
			CompanyName:      "Move Big Rocks",
			SiteTitle:        "Careers at Move Big Rocks",
			HeroTitle:        "Build products with leverage.",
			HeroBody:         "Thoughtful teams, real ownership.",
			JobsHeading:      "Open roles",
			TeamHeading:      "The team",
			GalleryHeading:   "How we work",
			PrimaryColor:     "#14532d",
			AccentColor:      "#f59e0b",
			SurfaceColor:     "#f5f7f0",
			BackgroundColor:  "#fbfcf8",
			TextColor:        "#12211b",
			MutedColor:       "#5f6b65",
			ContactEmail:     "careers@example.com",
			AddressCountry:   "NL",
			AddressLocality:  "Amsterdam",
			StreetAddress:    "101 Market Street",
			WebsiteURL:       "https://example.com",
			PrivacyPolicyURL: "https://example.com/privacy",
			CustomCSSEnabled: true,
			CustomCSS:        ".quote-block { color: red; }",
		},
		Jobs: []Vacancy{
			{
				Slug:        "principal-engineer",
				Title:       "Principal Engineer",
				Summary:     "Own the hardest systems.",
				AboutTheJob: "Lead architecture and product delivery.",
				Responsibilities: pq.StringArray{
					"Technical direction",
					"Cross-functional leadership",
				},
				Offers:         pq.StringArray{"Meaningful ownership"},
				Status:         atsdomain.VacancyStatusOpen,
				EmploymentType: atsdomain.EmploymentTypeFullTime,
				WorkMode:       atsdomain.WorkModeHybrid,
				CreatedAt:      time.Now().UTC(),
			},
		},
		ResumeUploadsEnabled: true,
	}

	files, err := renderCareersSite(bundle)
	require.NoError(t, err)
	require.Contains(t, string(files["site/index.html"]), "Principal Engineer")
	require.Contains(t, string(files["site/jobs/principal-engineer"]), "\"@type\":\"JobPosting\"")
	require.Contains(t, string(files["site/jobs/principal-engineer"]), "/careers/applications")
	require.Contains(t, string(files["site/index.html"]), "https://example.com/careers")
	require.Contains(t, string(files["site/jobs/principal-engineer"]), "https://example.com/careers/jobs/principal-engineer")
	require.Contains(t, string(files["site/index.html"]), "/careers/assets/custom.css")
	require.Contains(t, string(files["site/index.html"]), "Privacy")
	require.Equal(t, ".quote-block { color: red; }", string(files["site/assets/custom.css"]))
	require.Contains(t, string(files["site/assets/site.css"]), "--primary: #14532d;")
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

func TestATSGeneralApplicationsAndTalentPoolRouting(t *testing.T) {
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

	submission, err := runtime.Service.SubmitApplication(ctx, SubmitApplicationInput{
		WorkspaceID: workspace.ID,
		VacancySlug: generalApplicationVacancySlug,
		Submission: atsdomain.CandidateSubmission{
			FullName:  "Taylor Generalist",
			Email:     "taylor@example.com",
			CoverNote: "I can add leverage across product and operations.",
		},
	})
	require.NoError(t, err)
	require.Equal(t, generalApplicationVacancySlug, submission.Vacancy.Slug)
	require.Equal(t, atsdomain.VacancyKindGeneralApplication, submission.Vacancy.Kind)

	generalProfiles, err := runtime.Service.ListCandidates(ctx, workspace.ID, CandidateListOptions{Scope: CandidateListScopeGeneral})
	require.NoError(t, err)
	require.Len(t, generalProfiles, 1)
	require.Equal(t, generalApplicationsQueueSlug, generalProfiles[0].CaseQueueSlug)
	require.False(t, generalProfiles[0].IsTalentPool)

	_, err = runtime.Service.RouteCandidate(ctx, workspace.ID, submission.Application.ID, CandidateRouteInput{
		Destination: string(CandidateListScopeTalentPool),
		ActorName:   "ATS Admin",
		Note:        "Strong profile for future openings.",
	})
	require.NoError(t, err)

	talentPoolProfiles, err := runtime.Service.ListCandidates(ctx, workspace.ID, CandidateListOptions{Scope: CandidateListScopeTalentPool})
	require.NoError(t, err)
	require.Len(t, talentPoolProfiles, 1)
	require.True(t, talentPoolProfiles[0].IsTalentPool)
	require.Equal(t, talentPoolQueueSlug, talentPoolProfiles[0].CaseQueueSlug)

	caseObj, err := store.Cases().GetCase(ctx, submission.Application.CaseID)
	require.NoError(t, err)
	require.Contains(t, caseObj.Tags, talentPoolCaseTag)

	_, err = runtime.Service.RouteCandidate(ctx, workspace.ID, submission.Application.ID, CandidateRouteInput{
		Destination: "job_queue",
		ActorName:   "ATS Admin",
	})
	require.NoError(t, err)

	generalProfiles, err = runtime.Service.ListCandidates(ctx, workspace.ID, CandidateListOptions{Scope: CandidateListScopeGeneral})
	require.NoError(t, err)
	require.Len(t, generalProfiles, 1)
	require.Equal(t, generalApplicationsQueueSlug, generalProfiles[0].CaseQueueSlug)
	require.False(t, generalProfiles[0].IsTalentPool)
}

func TestATSCareersMediaUploadPublishesManagedAsset(t *testing.T) {
	storeIface, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	store := storeIface.(*platformsql.Store)
	ctx := context.Background()
	require.NoError(t, ApplyMigrations(ctx, store.SqlxDB()))

	artifactRoot := t.TempDir()
	runtime, err := NewRuntime(store, WithManagedArtifactPath(artifactRoot))
	require.NoError(t, err)
	t.Cleanup(func() {
		runtime.RulesEngine.Stop()
	})

	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))
	installATSExtensionForTest(t, ctx, store, workspace.ID)

	asset, err := runtime.Service.UploadCareersMediaAsset(ctx, workspace.ID, "logo", "brand-mark.png", "image/png", int64(len([]byte("pngdata"))), bytes.NewReader([]byte("pngdata")))
	require.NoError(t, err)
	require.Equal(t, "logo", asset.Purpose)
	require.Contains(t, asset.PublicURL, "/careers/assets/uploads/")

	assets, err := runtime.ATSStore.ListCareersMediaAssets(ctx, workspace.ID)
	require.NoError(t, err)
	require.Len(t, assets, 1)

	files, err := filepath.Glob(filepath.Join(artifactRoot, "workspaces", workspace.ID, "extensions", "ats", "surfaces", "website", "site", "assets", "uploads", "*-brand-mark.png"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
}

func TestATSPublicResumeUploadsReturnOpaqueSingleUseTokens(t *testing.T) {
	storeIface, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	store := storeIface.(*platformsql.Store)
	ctx := context.Background()
	require.NoError(t, ApplyMigrations(ctx, store.SqlxDB()))

	attachmentService := newTestAttachmentService(t)
	runtime, err := NewRuntime(store, WithAttachmentService(attachmentService))
	require.NoError(t, err)
	t.Cleanup(func() {
		runtime.RulesEngine.Stop()
	})

	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	job, err := runtime.Service.CreateJob(ctx, CreateJobInput{
		WorkspaceID: workspace.ID,
		Slug:        "platform-engineer",
		Title:       "Platform Engineer",
		Summary:     "Build durable systems.",
		Description: "Own the platform edge to edge.",
	})
	require.NoError(t, err)
	_, err = runtime.Service.PublishJob(ctx, workspace.ID, job.ID, time.Now().UTC())
	require.NoError(t, err)

	engine := runtimehttp.DefaultEngine()
	RegisterRoutes(engine, runtime.Handler)
	server := httptest.NewServer(engine)
	defer server.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "resume.pdf")
	require.NoError(t, err)
	_, err = part.Write([]byte("%PDF-1.4 test resume"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	uploadReq, err := http.NewRequest(http.MethodPost, server.URL+"/careers/attachments", body)
	require.NoError(t, err)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("X-MBR-Workspace-ID", workspace.ID)

	uploadResp, err := http.DefaultClient.Do(uploadReq)
	require.NoError(t, err)
	defer uploadResp.Body.Close()
	require.Equal(t, http.StatusCreated, uploadResp.StatusCode)

	var uploaded map[string]any
	require.NoError(t, json.NewDecoder(uploadResp.Body).Decode(&uploaded))
	uploadToken := uploaded["id"].(string)
	require.True(t, strings.HasPrefix(uploadToken, publicAttachmentUploadTokenPrefix))

	publicUpload, err := runtime.ATSStore.GetPublicAttachmentUpload(ctx, workspace.ID, uploadToken)
	require.NoError(t, err)
	require.NotEmpty(t, publicUpload.AttachmentID)

	payload, err := json.Marshal(map[string]any{
		"vacancySlug":        "platform-engineer",
		"fullName":           "Morgan Lee",
		"email":              "morgan@example.com",
		"resumeAttachmentId": uploadToken,
	})
	require.NoError(t, err)

	submitReq, err := http.NewRequest(http.MethodPost, server.URL+"/careers/applications", bytes.NewReader(payload))
	require.NoError(t, err)
	submitReq.Header.Set("Content-Type", "application/json")
	submitReq.Header.Set("X-MBR-Workspace-ID", workspace.ID)

	submitResp, err := http.DefaultClient.Do(submitReq)
	require.NoError(t, err)
	defer submitResp.Body.Close()
	require.Equal(t, http.StatusCreated, submitResp.StatusCode)

	var submitted map[string]any
	require.NoError(t, json.NewDecoder(submitResp.Body).Decode(&submitted))
	applicationID := submitted["application"].(map[string]any)["id"].(string)

	storedApplication, err := runtime.ATSStore.GetApplication(ctx, workspace.ID, applicationID)
	require.NoError(t, err)
	require.Equal(t, publicUpload.AttachmentID, storedApplication.SubmissionResumeAttachmentID)

	consumedUpload, err := runtime.ATSStore.GetPublicAttachmentUpload(ctx, workspace.ID, uploadToken)
	require.NoError(t, err)
	require.NotNil(t, consumedUpload.ConsumedAt)

	reuseReq, err := http.NewRequest(http.MethodPost, server.URL+"/careers/applications", bytes.NewReader(payload))
	require.NoError(t, err)
	reuseReq.Header.Set("Content-Type", "application/json")
	reuseReq.Header.Set("X-MBR-Workspace-ID", workspace.ID)

	reuseResp, err := http.DefaultClient.Do(reuseReq)
	require.NoError(t, err)
	defer reuseResp.Body.Close()
	require.Equal(t, http.StatusBadRequest, reuseResp.StatusCode)
}

func TestATSPublicSubmitIgnoresSpoofedSourceMetadata(t *testing.T) {
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
		WorkspaceID: workspace.ID,
		Slug:        "designer",
		Title:       "Designer",
		Summary:     "Shape the product.",
		Description: "Design calm systems.",
	})
	require.NoError(t, err)
	_, err = runtime.Service.PublishJob(ctx, workspace.ID, job.ID, time.Now().UTC())
	require.NoError(t, err)

	engine := runtimehttp.DefaultEngine()
	RegisterRoutes(engine, runtime.Handler)
	server := httptest.NewServer(engine)
	defer server.Close()

	payload, err := json.Marshal(map[string]any{
		"vacancySlug":      "designer",
		"fullName":         "Taylor Swift",
		"email":            "taylor@example.com",
		"sourceKind":       "api",
		"source":           "spoofed",
		"sourceRefId":      "spoofed-ref",
		"formSubmissionId": "spoofed-form",
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/careers/applications", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MBR-Workspace-ID", workspace.ID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var decoded map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	application := decoded["application"].(map[string]any)
	applicationID := application["id"].(string)
	_, exposedApplicant := decoded["applicant"]
	require.False(t, exposedApplicant)
	_, exposedVacancy := decoded["vacancy"]
	require.False(t, exposedVacancy)

	storedApplication, err := runtime.ATSStore.GetApplication(ctx, workspace.ID, applicationID)
	require.NoError(t, err)
	require.Equal(t, atsdomain.ApplicationSourceKindATSPublic, storedApplication.SourceKind)
	require.Equal(t, "careers_site", storedApplication.Source)
	require.Equal(t, "", storedApplication.SourceRefID)
	require.Equal(t, "", storedApplication.FormSubmissionID)
}

func TestATSSetupStatusRequiresRealOperatorConfiguration(t *testing.T) {
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

	status, err := runtime.Service.SetupStatus(ctx, workspace.ID)
	require.NoError(t, err)
	require.False(t, status.IsCompleted)

	completed := map[string]bool{}
	for _, step := range status.Steps {
		completed[step.Key] = step.Completed
	}
	require.False(t, completed["brand"])
	require.False(t, completed["company"])
	require.False(t, completed["team"])
	require.False(t, completed["jobs"])
	require.False(t, completed["publish"])
}

func TestATSApplicationSnapshotsPreservePerJobSubmissionData(t *testing.T) {
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

	firstJob, err := runtime.Service.CreateJob(ctx, CreateJobInput{
		WorkspaceID: workspace.ID,
		Slug:        "engineer-one",
		Title:       "Engineer One",
		Summary:     "First role.",
		Description: "First role description.",
	})
	require.NoError(t, err)
	secondJob, err := runtime.Service.CreateJob(ctx, CreateJobInput{
		WorkspaceID: workspace.ID,
		Slug:        "engineer-two",
		Title:       "Engineer Two",
		Summary:     "Second role.",
		Description: "Second role description.",
	})
	require.NoError(t, err)
	_, err = runtime.Service.PublishJob(ctx, workspace.ID, firstJob.ID, time.Now().UTC())
	require.NoError(t, err)
	_, err = runtime.Service.PublishJob(ctx, workspace.ID, secondJob.ID, time.Now().UTC())
	require.NoError(t, err)

	firstResume := createUploadedAttachment(t, ctx, store, workspace.ID, "candidate-first.pdf", []byte("%PDF-1.4 first"))
	secondResume := createUploadedAttachment(t, ctx, store, workspace.ID, "candidate-second.pdf", []byte("%PDF-1.4 second"))

	_, err = runtime.Service.SubmitApplication(ctx, SubmitApplicationInput{
		WorkspaceID: workspace.ID,
		VacancySlug: firstJob.Slug,
		Submission: atsdomain.CandidateSubmission{
			FullName:           "Jordan Example",
			Email:              "jordan@example.com",
			CoverNote:          "First application note.",
			ResumeAttachmentID: firstResume.ID,
		},
	})
	require.NoError(t, err)
	_, err = runtime.Service.SubmitApplication(ctx, SubmitApplicationInput{
		WorkspaceID: workspace.ID,
		VacancySlug: secondJob.Slug,
		Submission: atsdomain.CandidateSubmission{
			FullName:           "Jordan Example",
			Email:              "jordan@example.com",
			CoverNote:          "Second application note.",
			ResumeAttachmentID: secondResume.ID,
		},
	})
	require.NoError(t, err)

	profiles, err := runtime.Service.ListCandidates(ctx, workspace.ID, CandidateListOptions{})
	require.NoError(t, err)
	require.Len(t, profiles, 2)

	notesByVacancy := map[string]CandidateProfile{}
	for _, profile := range profiles {
		notesByVacancy[profile.Application.VacancyID] = profile
	}
	require.Equal(t, "First application note.", notesByVacancy[firstJob.ID].Application.SubmissionCoverNote)
	require.Equal(t, firstResume.ID, notesByVacancy[firstJob.ID].Application.SubmissionResumeAttachmentID)
	require.Equal(t, "Second application note.", notesByVacancy[secondJob.ID].Application.SubmissionCoverNote)
	require.Equal(t, secondResume.ID, notesByVacancy[secondJob.ID].Application.SubmissionResumeAttachmentID)
}

func TestRenderCareersSiteHidesDraftJobsAndTombstonesClosedJobs(t *testing.T) {
	now := time.Now().UTC()
	publishedAt := now.Add(-24 * time.Hour)
	bundle := &CareersSiteBundle{
		Site: CareersSiteProfile{
			CompanyName: "Move Big Rocks",
			SiteTitle:   "Careers at Move Big Rocks",
			WebsiteURL:  "https://example.com",
		},
		Jobs: []Vacancy{
			{
				Slug:        "open-role",
				Title:       "Open Role",
				Status:      atsdomain.VacancyStatusOpen,
				Kind:        atsdomain.VacancyKindJob,
				PublishedAt: &publishedAt,
				CreatedAt:   now,
			},
			{
				Slug:      "draft-role",
				Title:     "Draft Role",
				Status:    atsdomain.VacancyStatusDraft,
				Kind:      atsdomain.VacancyKindJob,
				CreatedAt: now,
			},
			{
				Slug:        "closed-role",
				Title:       "Closed Role",
				Status:      atsdomain.VacancyStatusClosed,
				Kind:        atsdomain.VacancyKindJob,
				PublishedAt: &publishedAt,
				CreatedAt:   now,
			},
		},
	}

	files, err := renderCareersSite(bundle)
	require.NoError(t, err)
	require.Contains(t, string(files["site/index.html"]), "Open Role")
	require.NotContains(t, string(files["site/index.html"]), "Draft Role")
	require.NotContains(t, string(files["site/index.html"]), "Closed Role")
	_, ok := files["site/jobs/draft-role"]
	require.False(t, ok)
	require.Contains(t, string(files["site/jobs/closed-role"]), "not currently published")
}

type testS3Server struct {
	server *httptest.Server
	mu     sync.Mutex
	puts   [][]byte
}

func newTestAttachmentService(t testing.TB) *serviceapp.AttachmentService {
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
	return attachmentService
}

func createUploadedAttachment(t *testing.T, ctx context.Context, store *platformsql.Store, workspaceID, filename string, payload []byte) *servicedomain.Attachment {
	t.Helper()

	attachmentService := newTestAttachmentService(t)

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

func installATSExtensionForTest(t testing.TB, ctx context.Context, store *platformsql.Store, workspaceID string) {
	t.Helper()
	_, filename, _, ok := goruntime.Caller(0)
	require.True(t, ok)
	manifestPath := filepath.Join(filepath.Dir(filename), "..", "manifest.json")
	body, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var manifest platformdomain.ExtensionManifest
	require.NoError(t, json.Unmarshal(body, &manifest))

	installed, err := platformdomain.NewInstalledExtension(workspaceID, "test-admin", "license-test", manifest, []byte("bundle"))
	require.NoError(t, err)
	require.NoError(t, store.Extensions().CreateInstalledExtension(ctx, installed))
}
