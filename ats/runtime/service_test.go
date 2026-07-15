package atsruntime

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/extension-sdk/extdb"
	"github.com/movebigrocks/extension-sdk/extensionhost/testutil"
	"github.com/movebigrocks/extension-sdk/runtimehost"
	atsdomain "github.com/movebigrocks/extensions/ats/runtime/domain"
)

// fakeHost is an in-memory stand-in for the platform host API. It records the
// calls the ATS runtime makes and holds just enough core state (queues, cases)
// for a flow to read back what it wrote. Core-side idempotency and rule firing
// are the platform's concern and are tested there; here we assert that ATS
// drives the host API with the right inputs and keeps its own tables coherent.
type fakeHost struct {
	seq          int
	queuesBySlug map[string]*runtimehost.HostQueue
	queuesByID   map[string]*runtimehost.HostQueue
	casesByID    map[string]*runtimehost.HostCase

	ingestByKey map[string]*runtimehost.IngestApplicationResult
	appliedKeys map[string]bool

	ingestCalls   []runtimehost.IngestApplicationInput
	applyCalls    []applyCall
	createdQueues []runtimehost.CreateQueueInput
	handoffs      []runtimehost.HandoffCaseInput
	updates       []updateCall
	uploads       []runtimehost.UploadAttachmentInput
	published     []runtimehost.PublishArtifactInput
	ruleFirings   int
}

type applyCall struct {
	caseID string
	input  runtimehost.ApplyCaseChangeInput
}

type updateCall struct {
	caseID string
	patch  runtimehost.CaseUpdateInput
}

func newFakeHost() *fakeHost {
	return &fakeHost{
		queuesBySlug: map[string]*runtimehost.HostQueue{},
		queuesByID:   map[string]*runtimehost.HostQueue{},
		casesByID:    map[string]*runtimehost.HostCase{},
		ingestByKey:  map[string]*runtimehost.IngestApplicationResult{},
		appliedKeys:  map[string]bool{},
	}
}

func (f *fakeHost) nextID(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s_%d", prefix, f.seq)
}

func (f *fakeHost) GetQueue(_ context.Context, queueID string) (*runtimehost.HostQueue, bool, error) {
	q, ok := f.queuesByID[queueID]
	return q, ok, nil
}

func (f *fakeHost) GetQueueBySlug(_ context.Context, slug string) (*runtimehost.HostQueue, bool, error) {
	q, ok := f.queuesBySlug[slug]
	return q, ok, nil
}

func (f *fakeHost) CreateQueue(_ context.Context, input runtimehost.CreateQueueInput) (*runtimehost.HostQueue, error) {
	f.createdQueues = append(f.createdQueues, input)
	if existing, ok := f.queuesBySlug[input.Slug]; ok {
		return existing, nil
	}
	q := &runtimehost.HostQueue{ID: f.nextID("queue"), Slug: input.Slug, Name: input.Name, Description: input.Description}
	f.queuesBySlug[input.Slug] = q
	f.queuesByID[q.ID] = q
	return q, nil
}

func (f *fakeHost) GetCase(_ context.Context, caseID string) (*runtimehost.HostCase, bool, error) {
	c, ok := f.casesByID[caseID]
	return c, ok, nil
}

func (f *fakeHost) UpdateCase(_ context.Context, caseID string, patch runtimehost.CaseUpdateInput) (*runtimehost.HostCase, error) {
	f.updates = append(f.updates, updateCall{caseID: caseID, patch: patch})
	c := f.casesByID[caseID]
	if c == nil {
		return nil, fmt.Errorf("case %s not found", caseID)
	}
	if patch.Tags != nil {
		c.Tags = *patch.Tags
	}
	if c.CustomFields == nil {
		c.CustomFields = map[string]any{}
	}
	for k, v := range patch.CustomFields {
		c.CustomFields[k] = v
	}
	return c, nil
}

func (f *fakeHost) HandoffCase(_ context.Context, caseID string, input runtimehost.HandoffCaseInput) error {
	f.handoffs = append(f.handoffs, input)
	if c := f.casesByID[caseID]; c != nil {
		c.QueueID = input.QueueID
	}
	return nil
}

func (f *fakeHost) UploadAttachment(_ context.Context, input runtimehost.UploadAttachmentInput) (*runtimehost.HostAttachment, error) {
	f.uploads = append(f.uploads, input)
	return &runtimehost.HostAttachment{
		ID:          f.nextID("att"),
		Filename:    input.Filename,
		ContentType: input.ContentType,
		Size:        int64(len(input.Content)),
		Status:      "clean",
	}, nil
}

func (f *fakeHost) GetAttachment(_ context.Context, attachmentID string) (*runtimehost.HostAttachment, bool, error) {
	return &runtimehost.HostAttachment{ID: attachmentID, Status: "clean"}, true, nil
}

func (f *fakeHost) LinkAttachmentsToCase(_ context.Context, _ string, _ []string) error {
	return nil
}

func (f *fakeHost) PublishArtifact(_ context.Context, input runtimehost.PublishArtifactInput) error {
	f.published = append(f.published, input)
	return nil
}

func (f *fakeHost) IngestApplication(_ context.Context, input runtimehost.IngestApplicationInput) (*runtimehost.IngestApplicationResult, error) {
	f.ingestCalls = append(f.ingestCalls, input)
	if prior, ok := f.ingestByKey[input.IdempotencyKey]; ok {
		return prior, nil
	}
	result := &runtimehost.IngestApplicationResult{ContactID: f.nextID("contact"), CaseID: f.nextID("case")}
	f.casesByID[result.CaseID] = &runtimehost.HostCase{
		ID:           result.CaseID,
		Subject:      input.Case.Subject,
		QueueID:      input.Case.QueueID,
		Tags:         input.Case.Tags,
		ContactID:    result.ContactID,
		CustomFields: cloneAnyMap(input.Case.CustomFields),
	}
	f.ingestByKey[input.IdempotencyKey] = result
	return result, nil
}

func (f *fakeHost) ApplyCaseChange(_ context.Context, caseID string, input runtimehost.ApplyCaseChangeInput) (*runtimehost.HostCase, error) {
	f.applyCalls = append(f.applyCalls, applyCall{caseID: caseID, input: input})
	c := f.casesByID[caseID]
	if c == nil {
		return nil, fmt.Errorf("case %s not found", caseID)
	}
	if f.appliedKeys[input.IdempotencyKey] {
		return c, nil
	}
	f.appliedKeys[input.IdempotencyKey] = true
	if c.CustomFields == nil {
		c.CustomFields = map[string]any{}
	}
	for k, v := range input.Patch.CustomFields {
		c.CustomFields[k] = v
	}
	if strings.TrimSpace(input.Event) != "" {
		f.ruleFirings++
	}
	return c, nil
}

func cloneAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

var _ coreHost = (*fakeHost)(nil)

func setupATS(t *testing.T) (*Service, *fakeHost, string) {
	t.Helper()
	dsn, cleanup := testutil.SetupTestPostgresDatabase(t)
	t.Cleanup(cleanup)

	db, err := extdb.Open(extdb.Config{DSN: dsn})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	// Migration 000004 backfills vacancy queue ids from core_service.case_queues,
	// a one-time coherence step that reads a core table present in the shared
	// production database. Tests run against a bare database and create fresh
	// vacancies, so an empty stub of that table lets the migration run as a
	// no-op without pulling in the whole core schema.
	_, err = db.Get(ctx).ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS core_service`)
	require.NoError(t, err)
	_, err = db.Get(ctx).ExecContext(ctx, `CREATE TABLE IF NOT EXISTS core_service.case_queues (id UUID, workspace_id UUID, slug TEXT, deleted_at TIMESTAMPTZ)`)
	require.NoError(t, err)
	require.NoError(t, ApplyMigrations(ctx, db))

	store, err := NewStore(db)
	require.NoError(t, err)

	fake := newFakeHost()
	svc := NewService(store, func(context.Context) (coreHost, error) { return fake, nil })
	return svc, fake, testutil.NewIsolatedWorkspace(t).ID
}

func sampleJob(workspaceID string) CreateJobInput {
	return CreateJobInput{
		WorkspaceID:    workspaceID,
		Slug:           "backend-engineer",
		Title:          "Backend Engineer",
		Team:           "Platform",
		Location:       "Amsterdam",
		WorkMode:       atsdomain.WorkModeHybrid,
		EmploymentType: atsdomain.EmploymentTypeFullTime,
		Summary:        "Own the API and data plane.",
		Description:    "Build and operate the recruiting runtime.",
		Language:       "en",
	}
}

func createPublishedJob(t *testing.T, svc *Service, ctx context.Context, workspaceID string) *Vacancy {
	t.Helper()
	job, err := svc.CreateJob(ctx, sampleJob(workspaceID))
	require.NoError(t, err)
	published, err := svc.PublishJob(ctx, workspaceID, job.ID, job.CreatedAt)
	require.NoError(t, err)
	return published
}

func sampleSubmission(workspaceID, vacancySlug, resumeID string) SubmitApplicationInput {
	return SubmitApplicationInput{
		WorkspaceID: workspaceID,
		VacancySlug: vacancySlug,
		Submission: atsdomain.CandidateSubmission{
			FullName:           "Ada Lovelace",
			Email:              "ada@example.com",
			Phone:              "+31 20 555 0100",
			Location:           "Amsterdam",
			CoverNote:          "I would like to help build the platform.",
			ResumeAttachmentID: resumeID,
			Source:             "careers_runtime_test",
		},
	}
}

func TestCreateJobCreatesCoreQueueAndBackfills(t *testing.T) {
	svc, fake, workspaceID := setupATS(t)
	ctx := context.Background()

	job, err := svc.CreateJob(ctx, sampleJob(workspaceID))
	require.NoError(t, err)
	require.Equal(t, "backend-engineer-candidates", job.CaseQueueSlug)
	require.NotEmpty(t, job.CaseQueueID)

	// Workspace provisioning seeds the default queues (general applications,
	// talent pool) through the host too, so assert the job's queue is among
	// those created rather than that it is the only one.
	slugs := make([]string, len(fake.createdQueues))
	for i, q := range fake.createdQueues {
		slugs[i] = q.Slug
	}
	require.Contains(t, slugs, "backend-engineer-candidates")

	// The queue id the host returned is persisted on the ATS vacancy row.
	reloaded, err := svc.store.GetVacancy(ctx, workspaceID, job.ID)
	require.NoError(t, err)
	require.Equal(t, job.CaseQueueID, reloaded.CaseQueueID)
}

func TestSubmitApplicationIngestsThroughHostAndPersistsLocally(t *testing.T) {
	svc, fake, workspaceID := setupATS(t)
	ctx := context.Background()

	job := createPublishedJob(t, svc, ctx, workspaceID)

	result, err := svc.SubmitApplication(ctx, sampleSubmission(workspaceID, job.Slug, "att_provided"))
	require.NoError(t, err)

	// The core contact and case ids come from the coarse host ingest.
	require.Len(t, fake.ingestCalls, 1)
	ingest := fake.ingestCalls[0]
	require.Equal(t, "ada@example.com", ingest.Contact.Email)
	require.Equal(t, job.CaseQueueID, ingest.Case.QueueID)
	require.Contains(t, ingest.Case.Tags, "job:"+job.Slug)
	require.Equal(t, []string{"att_provided"}, ingest.AttachmentIDs)
	require.Equal(t, fake.ingestByKey[ingest.IdempotencyKey].CaseID, result.Application.CaseID)
	require.Equal(t, fake.ingestByKey[ingest.IdempotencyKey].ContactID, result.Applicant.ContactID)

	// The ATS-owned application row is written from those ids.
	stored, err := svc.store.GetApplication(ctx, workspaceID, result.Application.ID)
	require.NoError(t, err)
	require.Equal(t, result.Application.CaseID, stored.CaseID)
	require.Equal(t, result.Applicant.ContactID, stored.ContactID)
}

func TestSubmitApplicationUsesStableIdempotencyKeyForRetries(t *testing.T) {
	svc, fake, workspaceID := setupATS(t)
	ctx := context.Background()

	job := createPublishedJob(t, svc, ctx, workspaceID)

	input := sampleSubmission(workspaceID, job.Slug, "")
	first, err := svc.SubmitApplication(ctx, input)
	require.NoError(t, err)
	require.Equal(t, submissionIdempotencyKey(input), fake.ingestCalls[0].IdempotencyKey)
	require.NotEmpty(t, first.Application.CaseID)

	// A byte-identical resubmission (a retry, a double click) must succeed
	// idempotently: the host dedups the contact and case onto the same ids, and
	// the ATS application row is returned rather than colliding on its unique
	// (workspace, vacancy, applicant) key.
	second, err := svc.SubmitApplication(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first.Application.ID, second.Application.ID)
	require.Equal(t, first.Application.CaseID, second.Application.CaseID)
	require.Equal(t, first.Applicant.ContactID, second.Applicant.ContactID)
	require.Len(t, fake.ingestByKey, 1) // exactly one core contact+case created

	// A submission that differs in any field gets a distinct key.
	other := sampleSubmission(workspaceID, job.Slug, "")
	other.Submission.Email = "grace@example.com"
	require.NotEqual(t, submissionIdempotencyKey(input), submissionIdempotencyKey(other))
}

func TestChangeCandidateStageAppliesCaseChangeIdempotently(t *testing.T) {
	svc, fake, workspaceID := setupATS(t)
	ctx := context.Background()

	job := createPublishedJob(t, svc, ctx, workspaceID)
	submitted, err := svc.SubmitApplication(ctx, sampleSubmission(workspaceID, job.Slug, ""))
	require.NoError(t, err)

	updated, err := svc.ChangeCandidateStage(ctx, workspaceID, submitted.Application.ID, StageChangeInput{
		Stage:     atsdomain.ApplicationStageScreening,
		ActorName: "Hiring Manager",
		Note:      "Screening booked.",
	})
	require.NoError(t, err)
	require.Equal(t, atsdomain.ApplicationStageScreening, updated.Stage)

	require.Len(t, fake.applyCalls, 1)
	change := fake.applyCalls[0]
	require.Equal(t, submitted.Application.CaseID, change.caseID)
	require.Equal(t, "ats_application_stage_changed", change.input.Event)
	require.Equal(t, string(atsdomain.ApplicationStageScreening), change.input.Patch.CustomFields["ats_application_stage"])
	require.Equal(t, 1, fake.ruleFirings)

	// The ATS application row reflects the new stage.
	stored, err := svc.store.GetApplication(ctx, workspaceID, submitted.Application.ID)
	require.NoError(t, err)
	require.Equal(t, atsdomain.ApplicationStageScreening, stored.Stage)
}

func TestUploadCareerAttachmentGoesThroughHost(t *testing.T) {
	svc, fake, workspaceID := setupATS(t)
	ctx := context.Background()

	resp, err := svc.UploadCareerAttachment(ctx, workspaceID, "cv.pdf", "application/pdf", "resume", 12, strings.NewReader("%PDF-1.4 cv"))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(resp.ID, publicAttachmentUploadTokenPrefix))
	require.Len(t, fake.uploads, 1)
	require.Equal(t, "cv.pdf", fake.uploads[0].Filename)
	require.Equal(t, []byte("%PDF-1.4 cv"), fake.uploads[0].Content)
}

func TestRouteCandidateHandsOffAndPatchesCase(t *testing.T) {
	svc, fake, workspaceID := setupATS(t)
	ctx := context.Background()

	job := createPublishedJob(t, svc, ctx, workspaceID)
	submitted, err := svc.SubmitApplication(ctx, sampleSubmission(workspaceID, job.Slug, ""))
	require.NoError(t, err)

	_, err = svc.RouteCandidate(ctx, workspaceID, submitted.Application.ID, CandidateRouteInput{
		Destination: string(CandidateListScopeTalentPool),
		ActorName:   "Recruiter",
		ActorType:   "recruiter",
	})
	require.NoError(t, err)

	require.Len(t, fake.handoffs, 1)
	require.Len(t, fake.updates, 1)
	require.Equal(t, talentPoolQueueSlug, fake.updates[0].patch.CustomFields["ats_candidate_bucket"])
	require.NotNil(t, fake.updates[0].patch.Tags)
	require.Contains(t, *fake.updates[0].patch.Tags, talentPoolCaseTag)
}
