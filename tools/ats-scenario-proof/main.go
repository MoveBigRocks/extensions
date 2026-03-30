package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
	"github.com/movebigrocks/extension-sdk/runtimehttp"
	atsruntime "github.com/movebigrocks/platform/extensions/ats/runtime"
	automationdomain "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"
	platformsql "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/id"
	"github.com/movebigrocks/platform/pkg/logger"
)

type proofResponse struct {
	Status int            `json:"status"`
	Body   map[string]any `json:"body,omitempty"`
}

type proofArtifact struct {
	Version    string                   `json:"version"`
	GitSHA     string                   `json:"gitSha"`
	BuildDate  string                   `json:"buildDate"`
	Workspace  map[string]string        `json:"workspace"`
	Requests   map[string]proofResponse `json:"requests"`
	Case       map[string]any           `json:"case"`
	Attachment map[string]any           `json:"attachment"`
	Defaults   map[string]any           `json:"defaults"`
}

func main() {
	var (
		outPath   string
		version   string
		gitSHA    string
		buildDate string
	)
	flag.StringVar(&outPath, "out", "", "path to the JSON proof artifact")
	flag.StringVar(&version, "version", "", "proof version")
	flag.StringVar(&gitSHA, "git-sha", "", "git sha")
	flag.StringVar(&buildDate, "build-date", "", "build date")
	flag.Parse()

	if outPath == "" {
		fmt.Fprintln(os.Stderr, "--out is required")
		os.Exit(2)
	}

	adminDSN, err := postgresAdminDSN()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve postgres admin dsn: %v\n", err)
		os.Exit(1)
	}
	testDSN, cleanupDB, err := createProofDatabase(adminDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create proof database: %v\n", err)
		os.Exit(1)
	}
	defer cleanupDB()

	db, err := platformsql.NewDBWithConfig(platformsql.DBConfig{DSN: testDSN})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open proof database: %v\n", err)
		os.Exit(1)
	}
	store, err := platformsql.NewStore(db)
	if err != nil {
		_ = db.Close()
		fmt.Fprintf(os.Stderr, "create proof store: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	ctx := context.Background()
	if err := atsruntime.ApplyMigrations(ctx, store.SqlxDB()); err != nil {
		fmt.Fprintf(os.Stderr, "apply ats migrations: %v\n", err)
		os.Exit(1)
	}

	runtime, err := atsruntime.NewRuntime(store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create ats runtime: %v\n", err)
		os.Exit(1)
	}
	defer runtime.RulesEngine.Stop()

	workspace := &platformdomain.Workspace{
		ID:        id.New(),
		Name:      "ATS Proof Workspace",
		Slug:      "ats-proof",
		ShortCode: "atsp",
	}
	if err := store.Workspaces().CreateWorkspace(ctx, workspace); err != nil {
		fmt.Fprintf(os.Stderr, "create proof workspace: %v\n", err)
		os.Exit(1)
	}

	rule := automationdomain.NewRule(workspace.ID, "ATS Stage Follow-up", "proof")
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
	if err := store.Rules().CreateRule(ctx, rule); err != nil {
		fmt.Fprintf(os.Stderr, "seed proof rule: %v\n", err)
		os.Exit(1)
	}

	resumeAttachment, err := createProofAttachment(ctx, store, workspace.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create proof attachment: %v\n", err)
		os.Exit(1)
	}

	engine := runtimehttp.DefaultEngine()
	atsruntime.RegisterRoutes(engine, runtime.Handler)
	server := httptest.NewServer(engine)
	defer server.Close()

	requests := map[string]proofResponse{}

	doJSON := func(name, method, path string, body any) map[string]any {
		var payload []byte
		if body != nil {
			encoded, err := json.Marshal(body)
			if err != nil {
				fmt.Fprintf(os.Stderr, "encode %s request: %v\n", name, err)
				os.Exit(1)
			}
			payload = encoded
		}
		req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(payload))
		if err != nil {
			fmt.Fprintf(os.Stderr, "build %s request: %v\n", name, err)
			os.Exit(1)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-MBR-Workspace-ID", workspace.ID)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "execute %s request: %v\n", name, err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		record := proofResponse{Status: resp.StatusCode}
		if resp.StatusCode != http.StatusNoContent {
			if err := json.NewDecoder(resp.Body).Decode(&record.Body); err != nil {
				fmt.Fprintf(os.Stderr, "decode %s response: %v\n", name, err)
				os.Exit(1)
			}
		}
		requests[name] = record
		if resp.StatusCode >= 400 {
			fmt.Fprintf(os.Stderr, "%s failed with %d: %#v\n", name, resp.StatusCode, record.Body)
			os.Exit(1)
		}
		return record.Body
	}

	created := doJSON("create_job", http.MethodPost, "/extensions/ats/api/jobs", map[string]any{
		"slug":           "backend-engineer",
		"title":          "Backend Engineer",
		"team":           "Platform",
		"location":       "Amsterdam",
		"workMode":       "hybrid",
		"employmentType": "full_time",
		"summary":        "Own the core API.",
		"description":    "Ship the ATS runtime and its proofs.",
	})
	jobID := created["id"].(string)

	doJSON("publish_job", http.MethodPost, "/extensions/ats/api/jobs/"+jobID+"/publish", nil)
	submitted := doJSON("submit_application", http.MethodPost, "/careers/applications", map[string]any{
		"vacancySlug":        "backend-engineer",
		"fullName":           "Ada Lovelace",
		"email":              "ada@example.com",
		"phone":              "+31 20 555 0100",
		"location":           "Amsterdam",
		"linkedinUrl":        "https://linkedin.example/ada",
		"portfolioUrl":       "https://portfolio.example/ada",
		"coverNote":          "I want to help build an agentic operations stack.",
		"resumeAttachmentId": resumeAttachment.ID,
		"source":             "milestone-proof",
	})

	application := submitted["application"].(map[string]any)
	applicationID := application["id"].(string)
	caseID := application["caseId"].(string)

	doJSON("add_note", http.MethodPost, "/extensions/ats/api/applications/"+applicationID+"/notes", map[string]any{
		"body":       "Strong profile, move to screening.",
		"authorName": "Hiring Manager",
		"authorType": "recruiter",
	})
	doJSON("stage_screening", http.MethodPost, "/extensions/ats/api/applications/"+applicationID+"/stage", map[string]any{
		"stage":     "screening",
		"actorName": "Hiring Manager",
		"note":      "Screening call booked.",
	})
	doJSON("stage_interview", http.MethodPost, "/extensions/ats/api/applications/"+applicationID+"/stage", map[string]any{
		"stage":     "interview",
		"actorName": "Hiring Manager",
		"note":      "Advance to panel interview.",
	})

	defaults := doJSON("workspace_defaults", http.MethodGet, "/extensions/ats/api/defaults", nil)
	doJSON("list_applications", http.MethodGet, "/extensions/ats/api/jobs/"+jobID+"/applications", nil)
	doJSON("close_job", http.MethodPost, "/extensions/ats/api/jobs/"+jobID+"/close", nil)
	doJSON("reopen_job", http.MethodPost, "/extensions/ats/api/jobs/"+jobID+"/reopen", nil)

	caseObj, err := store.Cases().GetCase(ctx, caseID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load proof case: %v\n", err)
		os.Exit(1)
	}
	linkedAttachment, err := store.Cases().GetAttachment(ctx, workspace.ID, resumeAttachment.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load proof attachment: %v\n", err)
		os.Exit(1)
	}
	visibleAttachments, err := store.Cases().ListCaseAttachments(ctx, workspace.ID, caseID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list proof case attachments: %v\n", err)
		os.Exit(1)
	}
	contact, err := store.Contacts().GetContactByEmail(ctx, workspace.ID, "ada@example.com")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load proof contact: %v\n", err)
		os.Exit(1)
	}

	artifact := proofArtifact{
		Version:   version,
		GitSHA:    gitSHA,
		BuildDate: buildDate,
		Workspace: map[string]string{
			"id":   workspace.ID,
			"slug": workspace.Slug,
		},
		Requests: requests,
		Case: map[string]any{
			"id":           caseObj.ID,
			"humanId":      caseObj.HumanID,
			"queueId":      caseObj.QueueID,
			"contactId":    contact.ID,
			"contact":      contact.Email,
			"tags":         caseObj.Tags,
			"subject":      caseObj.Subject,
			"customFields": caseObj.CustomFields.ToMap(),
		},
		Attachment: map[string]any{
			"id":           linkedAttachment.ID,
			"filename":     linkedAttachment.Filename,
			"status":       linkedAttachment.Status,
			"caseId":       linkedAttachment.CaseID,
			"storageKey":   linkedAttachment.S3Key,
			"visibleCount": len(visibleAttachments),
		},
		Defaults: defaults,
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create proof output dir: %v\n", err)
		os.Exit(1)
	}
	file, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create proof output: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(artifact); err != nil {
		fmt.Fprintf(os.Stderr, "write proof output: %v\n", err)
		os.Exit(1)
	}
}

func postgresAdminDSN() (string, error) {
	if value := os.Getenv("TEST_DATABASE_ADMIN_DSN"); value != "" {
		return value, nil
	}
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}
	if currentUser.Username == "" {
		return "", fmt.Errorf("TEST_DATABASE_ADMIN_DSN is not set and current user is empty")
	}
	return fmt.Sprintf("postgres://%s@127.0.0.1:5432/postgres?sslmode=disable", url.QueryEscape(currentUser.Username)), nil
}

func createProofDatabase(adminDSN string) (string, func(), error) {
	dbName := fmt.Sprintf("mbr_ats_proof_%d", time.Now().UnixNano())
	testDSN, err := postgresDSNWithDatabase(adminDSN, dbName)
	if err != nil {
		return "", nil, err
	}

	adminDB, err := sql.Open("postgres", adminDSN)
	if err != nil {
		return "", nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
		_ = adminDB.Close()
		return "", nil, err
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = adminDB.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName))
		_ = adminDB.Close()
	}
	return testDSN, cleanup, nil
}

func postgresDSNWithDatabase(adminDSN, databaseName string) (string, error) {
	parsed, err := url.Parse(adminDSN)
	if err != nil {
		return "", err
	}
	parsed.Path = "/" + databaseName
	return parsed.String(), nil
}

func createProofAttachment(ctx context.Context, store *platformsql.Store, workspaceID string) (*servicedomain.Attachment, error) {
	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer s3Server.Close()

	attachmentService, err := serviceapp.NewAttachmentService(serviceapp.AttachmentServiceConfig{
		S3Endpoint:  s3Server.URL,
		S3Region:    "us-east-1",
		S3Bucket:    "mbr-proof-attachments",
		S3AccessKey: "proof-access-key",
		S3SecretKey: "proof-secret-key",
		Logger:      logger.NewNop(),
	})
	if err != nil {
		return nil, fmt.Errorf("create attachment service: %w", err)
	}

	payload := []byte("%PDF-1.4 ats proof resume")
	attachment := servicedomain.NewAttachment(workspaceID, "ada-lovelace-resume.pdf", "application/pdf", int64(len(payload)), servicedomain.AttachmentSourceUpload)
	attachment.Description = "ATS scenario proof resume"
	if err := attachmentService.Upload(ctx, attachment, bytes.NewReader(payload)); err != nil {
		return nil, fmt.Errorf("upload proof attachment: %w", err)
	}
	if err := store.Cases().SaveAttachment(ctx, attachment, nil); err != nil {
		return nil, fmt.Errorf("persist proof attachment: %w", err)
	}
	return attachment, nil
}
