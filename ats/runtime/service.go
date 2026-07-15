package atsruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	atsdomain "github.com/movebigrocks/extensions/ats/runtime/domain"
)

type Service struct {
	store   *Store
	newHost hostProvider
}

const (
	generalApplicationsQueueSlug  = "general-applications"
	talentPoolQueueSlug           = "talent-pool"
	generalApplicationVacancySlug = "general-application"
	talentPoolCaseTag             = "ats-talent-pool"
)

// NewService builds the ATS service over its own store and a provider that
// yields the platform host API bound to each request's context. Core data
// (contacts, cases, queues, attachments, rules, artifacts) is reached only
// through that host API, never by importing platform internals.
func NewService(store *Store, newHost hostProvider) *Service {
	return &Service{store: store, newHost: newHost}
}

func (s *Service) ensureWorkspaceProvisioned(ctx context.Context, workspaceID string) (*WorkspaceDefaults, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	defaults, err := s.store.EnsureWorkspaceDefaults(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if _, err := s.ensureRoutingQueue(ctx, workspaceID, generalApplicationsQueueSlug, "General Applications", "Default queue for incoming general applications."); err != nil {
		return nil, err
	}
	if _, err := s.ensureRoutingQueue(ctx, workspaceID, talentPoolQueueSlug, "Talent Pool", "Reusable queue for strong candidates who should stay warm."); err != nil {
		return nil, err
	}
	if _, err := s.ensureGeneralApplicationVacancy(ctx, workspaceID); err != nil {
		return nil, err
	}
	return defaults, nil
}

func (s *Service) ensureRoutingQueue(ctx context.Context, workspaceID, slug, name, description string) (*runtimehost.HostQueue, error) {
	host, err := s.newHost(ctx)
	if err != nil {
		return nil, err
	}
	if queue, found, err := host.GetQueueBySlug(ctx, slug); err != nil {
		return nil, fmt.Errorf("load queue %s: %w", slug, err)
	} else if found {
		return queue, nil
	}
	queue, err := host.CreateQueue(ctx, runtimehost.CreateQueueInput{
		Name:        strings.TrimSpace(name),
		Slug:        strings.TrimSpace(slug),
		Description: strings.TrimSpace(description),
	})
	if err == nil {
		return queue, nil
	}
	if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		return nil, fmt.Errorf("create queue %s: %w", slug, err)
	}
	queue, found, err := host.GetQueueBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("load queue %s: %w", slug, err)
	}
	if !found {
		return nil, fmt.Errorf("queue %s vanished after a duplicate create", slug)
	}
	return queue, nil
}

func (s *Service) ensureGeneralApplicationVacancy(ctx context.Context, workspaceID string) (*Vacancy, error) {
	queue, err := s.ensureRoutingQueue(ctx, workspaceID, generalApplicationsQueueSlug, "General Applications", "Default queue for incoming general applications.")
	if err != nil {
		return nil, err
	}
	existing, err := s.store.GetVacancyBySlug(ctx, workspaceID, generalApplicationVacancySlug)
	if err == nil {
		domainVacancy := existing.toDomain()
		changed := false
		if domainVacancy.Kind != atsdomain.VacancyKindGeneralApplication {
			domainVacancy.Kind = atsdomain.VacancyKindGeneralApplication
			changed = true
		}
		if domainVacancy.CaseQueueID != queue.ID {
			domainVacancy.CaseQueueID = queue.ID
			changed = true
		}
		if domainVacancy.CaseQueueSlug != generalApplicationsQueueSlug {
			domainVacancy.CaseQueueSlug = generalApplicationsQueueSlug
			changed = true
		}
		if domainVacancy.CareersPath != "/careers#general-application" {
			domainVacancy.CareersPath = "/careers#general-application"
			changed = true
		}
		if domainVacancy.Status != atsdomain.VacancyStatusOpen {
			if publishErr := domainVacancy.Publish(time.Now().UTC()); publishErr == nil {
				changed = true
			}
		}
		if strings.TrimSpace(domainVacancy.Summary) == "" {
			domainVacancy.Summary = "Send a thoughtful general application if the right role is not live yet."
			changed = true
		}
		if strings.TrimSpace(domainVacancy.Description) == "" {
			domainVacancy.Description = "We review general applications and route strong candidates into the right conversations as roles open."
			changed = true
		}
		if changed {
			return s.store.SaveVacancy(ctx, vacancyFromDomain(domainVacancy))
		}
		return existing, nil
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not found") {
		return nil, err
	}
	vacancy, err := atsdomain.NewVacancy(workspaceID, generalApplicationVacancySlug, "General Application")
	if err != nil {
		return nil, err
	}
	vacancy.Kind = atsdomain.VacancyKindGeneralApplication
	vacancy.Team = "Hiring"
	vacancy.Summary = "Send a thoughtful general application if the right role is not live yet."
	vacancy.Description = "We review general applications and route strong candidates into the right conversations as roles open."
	vacancy.AboutTheJob = "Use this route when there is not an exact job match yet, but you believe you can add leverage."
	vacancy.PublicLanguage = "en"
	vacancy.CaseQueueID = queue.ID
	vacancy.CaseQueueSlug = generalApplicationsQueueSlug
	vacancy.CareersPath = "/careers#general-application"
	if err := vacancy.Publish(time.Now().UTC()); err != nil {
		return nil, err
	}
	return s.store.InsertVacancy(ctx, vacancyFromDomain(vacancy))
}

func (s *Service) CreateJob(ctx context.Context, input CreateJobInput) (*Vacancy, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	host, err := s.newHost(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.ensureWorkspaceProvisioned(ctx, input.WorkspaceID); err != nil {
		return nil, err
	}

	// Create the ATS vacancy first so its queue slug is known, then create the
	// matching core queue through the host API, then backfill the queue id.
	// Keeping the host call outside a database transaction avoids holding an
	// ATS transaction open across a network round trip. A retry after a partial
	// failure is safe: a duplicate queue resolves to the existing one.
	vacancy, err := s.store.CreateVacancy(ctx, input)
	if err != nil {
		return nil, err
	}
	queue, err := host.CreateQueue(ctx, runtimehost.CreateQueueInput{
		Name:        vacancy.Title + " Candidates",
		Slug:        vacancy.CaseQueueSlug,
		Description: "Candidate review queue for " + vacancy.Title,
	})
	if err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, fmt.Errorf("create vacancy queue: %w", err)
		}
		existing, found, gerr := host.GetQueueBySlug(ctx, vacancy.CaseQueueSlug)
		if gerr != nil {
			return nil, fmt.Errorf("load existing vacancy queue: %w", gerr)
		}
		if !found {
			return nil, fmt.Errorf("vacancy queue %s vanished after a duplicate create", vacancy.CaseQueueSlug)
		}
		queue = existing
	}
	vacancy.CaseQueueID = queue.ID
	created, err := s.store.SaveVacancy(ctx, vacancy)
	if err != nil {
		return nil, err
	}
	if err := s.setSetupStepConfirmed(ctx, input.WorkspaceID, "jobs", true); err != nil {
		return nil, err
	}
	if err := s.publishCareersSiteIfInstalled(ctx, input.WorkspaceID); err != nil {
		return nil, err
	}
	return created, nil
}

func (s *Service) ListJobs(ctx context.Context, workspaceID string) ([]Vacancy, error) {
	if _, err := s.ensureWorkspaceProvisioned(ctx, workspaceID); err != nil {
		return nil, err
	}
	jobs, err := s.store.ListVacancies(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return filterPrimaryJobs(jobs), nil
}

func (s *Service) UpdateJob(ctx context.Context, workspaceID, vacancyID string, input UpdateJobInput) (*Vacancy, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	if _, err := s.ensureWorkspaceProvisioned(ctx, workspaceID); err != nil {
		return nil, err
	}
	vacancy, err := s.store.UpdateVacancy(ctx, workspaceID, vacancyID, input)
	if err != nil {
		return nil, err
	}
	if err := s.publishCareersSiteIfInstalled(ctx, workspaceID); err != nil {
		return nil, err
	}
	return vacancy, nil
}

func (s *Service) PublishJob(ctx context.Context, workspaceID, vacancyID string, at time.Time) (*Vacancy, error) {
	return s.updateVacancyState(ctx, workspaceID, vacancyID, func(vacancy *atsdomain.Vacancy) error {
		return vacancy.Publish(at)
	})
}

func (s *Service) CloseJob(ctx context.Context, workspaceID, vacancyID string, at time.Time) (*Vacancy, error) {
	return s.updateVacancyState(ctx, workspaceID, vacancyID, func(vacancy *atsdomain.Vacancy) error {
		return vacancy.Close(at)
	})
}

func (s *Service) ReopenJob(ctx context.Context, workspaceID, vacancyID string, at time.Time) (*Vacancy, error) {
	return s.updateVacancyState(ctx, workspaceID, vacancyID, func(vacancy *atsdomain.Vacancy) error {
		return vacancy.Reopen(at)
	})
}

func (s *Service) ListCandidates(ctx context.Context, workspaceID string, options CandidateListOptions) ([]CandidateProfile, error) {
	if _, err := s.ensureWorkspaceProvisioned(ctx, workspaceID); err != nil {
		return nil, err
	}
	profiles, err := s.store.ListCandidateProfiles(ctx, workspaceID, options.VacancyID)
	if err != nil {
		return nil, err
	}
	return s.enrichAndFilterCandidateProfiles(ctx, workspaceID, profiles, options)
}

func (s *Service) WorkspaceDefaults(ctx context.Context, workspaceID string) (*WorkspaceDefaults, error) {
	return s.ensureWorkspaceProvisioned(ctx, workspaceID)
}

func (s *Service) ReplaceStagePresets(ctx context.Context, workspaceID string, presets []StagePreset) ([]StagePreset, error) {
	if _, err := s.ensureWorkspaceProvisioned(ctx, workspaceID); err != nil {
		return nil, err
	}
	return s.store.ReplaceStagePresets(ctx, workspaceID, presets)
}

func (s *Service) ReplaceSavedViews(ctx context.Context, workspaceID string, filters []SavedFilter) ([]SavedFilter, error) {
	if _, err := s.ensureWorkspaceProvisioned(ctx, workspaceID); err != nil {
		return nil, err
	}
	return s.store.ReplaceSavedFilters(ctx, workspaceID, filters)
}

func (s *Service) SetupStatus(ctx context.Context, workspaceID string) (*SetupStatus, error) {
	if _, err := s.ensureWorkspaceProvisioned(ctx, workspaceID); err != nil {
		return nil, err
	}
	site, err := s.store.GetCareersSiteProfile(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	team, err := s.store.ListCareersTeamMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	gallery, err := s.store.ListCareersGalleryItems(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	jobs, err := s.store.ListVacancies(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.syncSetupStatus(ctx, workspaceID, site, team, gallery, jobs)
}

func (s *Service) SaveSetupState(ctx context.Context, workspaceID, currentStep string) (*SetupStatus, error) {
	if _, err := s.ensureWorkspaceProvisioned(ctx, workspaceID); err != nil {
		return nil, err
	}
	existing, err := s.store.GetCareersSetupState(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	state, err := s.store.SaveCareersSetupState(ctx, CareersSetupState{
		WorkspaceID:    workspaceID,
		CurrentStep:    currentStep,
		ConfirmedSteps: existing.ConfirmedSteps,
		CompletedAt:    existing.CompletedAt,
		CreatedAt:      existing.CreatedAt,
	})
	if err != nil {
		return nil, err
	}
	site, err := s.store.GetCareersSiteProfile(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	team, err := s.store.ListCareersTeamMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	gallery, err := s.store.ListCareersGalleryItems(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	jobs, err := s.store.ListVacancies(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.syncSetupStatusWithState(ctx, state, site, team, gallery, jobs)
}

func (s *Service) CareersSiteBundle(ctx context.Context, workspaceID string) (*CareersSiteBundle, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	if _, err := s.ensureWorkspaceProvisioned(ctx, workspaceID); err != nil {
		return nil, err
	}
	site, err := s.store.GetCareersSiteProfile(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	team, err := s.store.ListCareersTeamMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	gallery, err := s.store.ListCareersGalleryItems(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	assets, err := s.store.ListCareersMediaAssets(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	jobs, err := s.store.ListVacancies(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	setup, err := s.syncSetupStatus(ctx, workspaceID, site, team, gallery, jobs)
	if err != nil {
		return nil, err
	}
	return &CareersSiteBundle{
		Site:                 *site,
		Team:                 team,
		Gallery:              gallery,
		Assets:               assets,
		Jobs:                 jobs,
		Setup:                *setup,
		PreviewURL:           "/careers",
		ResumeUploadsEnabled: true,
	}, nil
}

func (s *Service) SaveCareersSiteProfile(ctx context.Context, input UpsertCareersSiteInput) (*CareersSiteProfile, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	if _, err := s.ensureWorkspaceProvisioned(ctx, input.WorkspaceID); err != nil {
		return nil, err
	}
	profile, err := s.store.SaveCareersSiteProfile(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.updateSiteSetupConfirmations(ctx, input.WorkspaceID, profile); err != nil {
		return nil, err
	}
	if err := s.publishCareersSiteIfInstalled(ctx, input.WorkspaceID); err != nil {
		return nil, err
	}
	return profile, nil
}

func (s *Service) ReplaceCareersTeamMembers(ctx context.Context, workspaceID string, members []CareersTeamMember) ([]CareersTeamMember, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	if _, err := s.ensureWorkspaceProvisioned(ctx, workspaceID); err != nil {
		return nil, err
	}
	saved, err := s.store.ReplaceCareersTeamMembers(ctx, workspaceID, members)
	if err != nil {
		return nil, err
	}
	gallery, err := s.store.ListCareersGalleryItems(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.updateTeamSetupConfirmation(ctx, workspaceID, saved, gallery); err != nil {
		return nil, err
	}
	if err := s.publishCareersSiteIfInstalled(ctx, workspaceID); err != nil {
		return nil, err
	}
	return saved, nil
}

func (s *Service) ReplaceCareersGalleryItems(ctx context.Context, workspaceID string, items []CareersGalleryItem) ([]CareersGalleryItem, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	if _, err := s.ensureWorkspaceProvisioned(ctx, workspaceID); err != nil {
		return nil, err
	}
	saved, err := s.store.ReplaceCareersGalleryItems(ctx, workspaceID, items)
	if err != nil {
		return nil, err
	}
	team, err := s.store.ListCareersTeamMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.updateTeamSetupConfirmation(ctx, workspaceID, team, saved); err != nil {
		return nil, err
	}
	if err := s.publishCareersSiteIfInstalled(ctx, workspaceID); err != nil {
		return nil, err
	}
	return saved, nil
}

func (s *Service) PublishCareersSite(ctx context.Context, workspaceID string) error {
	if err := s.publishCareersSite(ctx, workspaceID, false); err != nil {
		return err
	}
	return s.setSetupStepConfirmed(ctx, workspaceID, "publish", true)
}

func (s *Service) SubmitApplication(ctx context.Context, input SubmitApplicationInput) (*SubmissionResult, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	host, err := s.newHost(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.ensureWorkspaceProvisioned(ctx, input.WorkspaceID); err != nil {
		return nil, err
	}

	vacancy, err := s.store.GetVacancyBySlug(ctx, input.WorkspaceID, input.VacancySlug)
	if err != nil {
		return nil, err
	}
	resolvedResumeAttachmentID, publicUpload, err := s.resolveSubmissionResumeAttachment(ctx, input.WorkspaceID, input.Submission.ResumeAttachmentID)
	if err != nil {
		return nil, err
	}
	submission := input.Submission
	submission.ResumeAttachmentID = resolvedResumeAttachmentID
	applicantDomain, applicationDomain, err := atsdomain.BuildCandidateRecord(input.WorkspaceID, vacancy.toDomain(), submission)
	if err != nil {
		return nil, err
	}
	applicant := applicantFromDomain(applicantDomain)
	application := applicationFromDomain(applicationDomain)

	queue, err := s.resolveVacancyQueue(ctx, vacancy)
	if err != nil {
		return nil, err
	}

	customFields := map[string]any{}
	for key, value := range vacancy.toDomain().CaseCustomFields() {
		customFields[key] = value
	}
	for key, value := range applicantDomain.CaseCustomFields() {
		customFields[key] = value
	}
	for key, value := range applicationDomain.CaseCustomFields() {
		customFields[key] = value
	}
	customFields["ats_case_queue_id"] = queue.ID
	customFields["ats_case_queue_slug"] = queue.Slug
	if vacancy.Kind == atsdomain.VacancyKindGeneralApplication {
		customFields["ats_candidate_bucket"] = generalApplicationsQueueSlug
	} else {
		customFields["ats_candidate_bucket"] = "job_queue"
	}

	tags := []string{"ats", "candidate", "application", "applied"}
	if vacancy.Kind == atsdomain.VacancyKindGeneralApplication {
		tags = append(tags, "general-application")
	} else {
		tags = append(tags, "job:"+vacancy.Slug)
	}

	var attachmentIDs []string
	if id := strings.TrimSpace(application.SubmissionResumeAttachmentID); id != "" {
		attachmentIDs = []string{id}
	}

	// Create the applicant contact and candidate case in core through one
	// idempotent host operation, keyed by the submission's content so a retry
	// (a double click, a network retry) returns the same ids rather than
	// creating a second contact and case. The ATS-owned rows are written after,
	// in their own transaction, from the ids the host returns. The host call is
	// kept outside that transaction so no ATS transaction is held open across a
	// network round trip.
	ingest, err := host.IngestApplication(ctx, runtimehost.IngestApplicationInput{
		IdempotencyKey: submissionIdempotencyKey(input),
		Contact: runtimehost.CreateContactInput{
			Email:    applicantDomain.Email,
			Name:     applicantDomain.FullName,
			Phone:    applicantDomain.Phone,
			Source:   "ats",
			Metadata: map[string]any{"ats_vacancy_slug": vacancy.Slug},
		},
		Case: runtimehost.IngestCaseInput{
			Subject:      fmt.Sprintf("%s for %s", applicant.FullName, vacancy.Title),
			Description:  application.SubmissionCoverNote,
			QueueID:      queue.ID,
			Category:     "recruiting",
			Tags:         tags,
			CustomFields: customFields,
		},
		AttachmentIDs: attachmentIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("ingest application into core: %w", err)
	}

	var result *SubmissionResult
	err = s.store.WithTransaction(ctx, func(txCtx context.Context) error {
		applicant.ContactID = ingest.ContactID
		savedApplicant, err := s.store.UpsertApplicant(txCtx, applicant)
		if err != nil {
			return err
		}
		application.ApplicantID = savedApplicant.ID
		application.ContactID = ingest.ContactID
		application.CaseID = ingest.CaseID
		savedApplication, err := s.store.CreateApplication(txCtx, application)
		if err != nil {
			return err
		}
		if publicUpload != nil {
			if _, err := s.store.ConsumePublicAttachmentUpload(txCtx, input.WorkspaceID, publicUpload.Token); err != nil {
				return err
			}
		}
		result = &SubmissionResult{
			Vacancy:     *vacancy,
			Applicant:   *savedApplicant,
			Application: *savedApplication,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// submissionIdempotencyKey derives a stable key from a submission's content so
// the coarse core ingest deduplicates retries of the same submission. Two
// distinct submissions differ in at least one field and so get distinct keys;
// a byte-identical resubmission is indistinguishable from a retry and folds
// onto the same key by design.
func submissionIdempotencyKey(input SubmitApplicationInput) string {
	raw, _ := json.Marshal(input.Submission)
	sum := sha256.Sum256([]byte(strings.Join([]string{input.WorkspaceID, input.VacancySlug, string(raw)}, "\x00")))
	return "ats_submit_" + hex.EncodeToString(sum[:])
}

func (s *Service) UploadCareerAttachment(ctx context.Context, workspaceID, filename, contentType, description string, size int64, reader io.Reader) (*PublicAttachmentUploadResponse, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("resume uploads are not configured")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	host, err := s.newHost(ctx)
	if err != nil {
		return nil, err
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read resume upload: %w", err)
	}
	attachment, err := host.UploadAttachment(ctx, runtimehost.UploadAttachmentInput{
		Filename:    strings.TrimSpace(filename),
		ContentType: strings.TrimSpace(contentType),
		Description: strings.TrimSpace(description),
		Content:     content,
	})
	if err != nil {
		return nil, fmt.Errorf("upload resume attachment: %w", err)
	}
	upload, err := s.store.CreatePublicAttachmentUpload(ctx, workspaceID, attachment.ID, "resume")
	if err != nil {
		return nil, err
	}
	return &PublicAttachmentUploadResponse{
		ID:          upload.Token,
		Filename:    attachment.Filename,
		ContentType: attachment.ContentType,
		Size:        attachment.Size,
		Status:      attachment.Status,
	}, nil
}

func (s *Service) UploadCareersMediaAsset(ctx context.Context, workspaceID, purpose, filename, contentType string, size int64, reader io.Reader) (*CareersMediaAsset, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("careers media publishing is not configured")
	}
	host, err := s.newHost(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.ensureWorkspaceProvisioned(ctx, workspaceID); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	filename = strings.TrimSpace(filename)
	contentType = strings.TrimSpace(strings.ToLower(contentType))
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	if !strings.HasPrefix(contentType, "image/") {
		return nil, fmt.Errorf("careers media must be an image upload")
	}
	payload, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read media upload: %w", err)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("media upload is empty")
	}
	if size <= 0 {
		size = int64(len(payload))
	}
	if size > 10*1024*1024 {
		return nil, fmt.Errorf("media upload exceeds the 10MB limit")
	}
	safeName := sanitizeMediaFilename(filename)
	assetID := "media_" + strings.ReplaceAll(strings.TrimSuffix(safeName, filepath.Ext(safeName)), ".", "-")
	if len(assetID) > 64 {
		assetID = assetID[:64]
	}
	assetPath := path.Join("site/assets/uploads", newATSAssetFilename(safeName))
	if err := host.PublishArtifact(ctx, runtimehost.PublishArtifactInput{
		Surface:      "website",
		RelativePath: assetPath,
		Content:      payload,
		ActorID:      "ats-runtime",
	}); err != nil {
		return nil, fmt.Errorf("publish careers media asset: %w", err)
	}
	publicURL := "/careers/" + strings.TrimPrefix(assetPath, "site/")
	return s.store.SaveCareersMediaAsset(ctx, CareersMediaAsset{
		ID:           assetID + "_" + fmt.Sprintf("%d", time.Now().UTC().UnixNano()),
		WorkspaceID:  workspaceID,
		Purpose:      purpose,
		Filename:     safeName,
		ContentType:  contentType,
		SizeBytes:    size,
		ArtifactPath: assetPath,
		PublicURL:    publicURL,
	})
}

func (s *Service) resolveSubmissionResumeAttachment(ctx context.Context, workspaceID, reference string) (string, *PublicAttachmentUpload, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", nil, nil
	}
	if !strings.HasPrefix(reference, publicAttachmentUploadTokenPrefix) {
		return reference, nil, nil
	}
	upload, err := s.store.GetPublicAttachmentUpload(ctx, workspaceID, reference)
	if err != nil {
		return "", nil, fmt.Errorf("resolve resume upload token: %w", err)
	}
	if upload.ConsumedAt != nil {
		return "", nil, fmt.Errorf("resume upload token has already been used")
	}
	return strings.TrimSpace(upload.AttachmentID), upload, nil
}

func (s *Service) AddRecruiterNote(ctx context.Context, workspaceID, applicationID, body, authorName, authorType string) (*RecruiterNote, error) {
	return s.store.AddRecruiterNote(ctx, workspaceID, applicationID, authorName, authorType, body)
}

func (s *Service) ChangeCandidateStage(ctx context.Context, workspaceID, applicationID string, input StageChangeInput) (*Application, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(applicationID) == "" {
		return nil, fmt.Errorf("workspace ID and application ID are required")
	}
	host, err := s.newHost(ctx)
	if err != nil {
		return nil, err
	}

	var (
		saved         *Application
		previousStage string
	)
	err = s.store.WithTransaction(ctx, func(txCtx context.Context) error {
		current, err := s.store.GetApplication(txCtx, workspaceID, applicationID)
		if err != nil {
			return err
		}
		previousStage = string(current.Stage)
		domainApp := current.toDomain()
		switch input.Stage {
		case atsdomain.ApplicationStageRejected:
			err = domainApp.Reject(input.Reason, occurredAt(input.OccurredAt))
		case atsdomain.ApplicationStageHired:
			err = domainApp.Hire(occurredAt(input.OccurredAt))
		case atsdomain.ApplicationStageWithdrawn:
			err = domainApp.Withdraw(occurredAt(input.OccurredAt))
		default:
			err = domainApp.AdvanceTo(input.Stage, occurredAt(input.OccurredAt))
		}
		if err != nil {
			return err
		}
		saved, err = s.store.SaveApplication(txCtx, applicationFromDomain(domainApp))
		if err != nil {
			return err
		}
		if strings.TrimSpace(input.Note) != "" {
			if _, err := s.store.AddRecruiterNote(txCtx, workspaceID, applicationID, input.ActorName, actorType(input.ActorType), input.Note); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Mirror the new stage onto the core case and fire stage-change automation
	// through one idempotent host operation, keyed by the application and its
	// new stage so a duplicate or retried transition does not re-fire the rules.
	if strings.TrimSpace(saved.CaseID) != "" {
		patch := runtimehost.CaseUpdateInput{
			CustomFields: map[string]any{"ats_application_stage": string(saved.Stage)},
		}
		if saved.RejectionReason != "" {
			patch.CustomFields["ats_application_rejection_reason"] = saved.RejectionReason
		}
		if _, err := host.ApplyCaseChange(ctx, saved.CaseID, runtimehost.ApplyCaseChangeInput{
			IdempotencyKey: fmt.Sprintf("ats_stage_%s_%s", saved.ID, saved.Stage),
			Patch:          patch,
			Event:          "ats_application_stage_changed",
			Changes: map[string]any{
				"ats_application_previous_stage": previousStage,
				"ats_application_stage":          string(saved.Stage),
			},
		}); err != nil {
			return nil, fmt.Errorf("apply candidate stage change to core: %w", err)
		}
	}
	return saved, nil
}

func (s *Service) RouteCandidate(ctx context.Context, workspaceID, applicationID string, input CandidateRouteInput) (*Application, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	host, err := s.newHost(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.ensureWorkspaceProvisioned(ctx, workspaceID); err != nil {
		return nil, err
	}
	destination := strings.TrimSpace(strings.ToLower(input.Destination))
	if destination == "" {
		return nil, fmt.Errorf("destination is required")
	}

	application, err := s.store.GetApplication(ctx, workspaceID, applicationID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(application.CaseID) == "" {
		return nil, fmt.Errorf("application %s is not linked to a candidate case", application.ID)
	}

	vacancy, err := s.store.GetVacancy(ctx, workspaceID, application.VacancyID)
	if err != nil {
		return nil, err
	}

	var targetQueue *runtimehost.HostQueue
	switch destination {
	case string(CandidateListScopeTalentPool):
		targetQueue, err = s.ensureRoutingQueue(ctx, workspaceID, talentPoolQueueSlug, "Talent Pool", "Reusable queue for strong candidates who should stay warm.")
	case "job_queue":
		targetQueue, err = s.resolveVacancyQueue(ctx, vacancy)
	default:
		return nil, fmt.Errorf("unsupported route destination %q", destination)
	}
	if err != nil {
		return nil, err
	}

	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		if destination == string(CandidateListScopeTalentPool) {
			reason = "Moved to talent pool."
		} else {
			reason = "Returned to the job queue."
		}
	}

	if err := host.HandoffCase(ctx, application.CaseID, runtimehost.HandoffCaseInput{
		QueueID:         targetQueue.ID,
		Reason:          reason,
		PerformedByName: strings.TrimSpace(input.ActorName),
		PerformedByType: actorType(input.ActorType),
	}); err != nil {
		return nil, fmt.Errorf("route candidate case: %w", err)
	}

	caseObj, found, err := host.GetCase(ctx, application.CaseID)
	if err != nil {
		return nil, fmt.Errorf("load routed candidate case: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("routed candidate case %s not found", application.CaseID)
	}
	customFields := map[string]any{
		"ats_case_queue_id":   targetQueue.ID,
		"ats_case_queue_slug": targetQueue.Slug,
	}
	tags := caseObj.Tags
	if destination == string(CandidateListScopeTalentPool) {
		customFields["ats_candidate_bucket"] = talentPoolQueueSlug
		tags = appendUniqueTag(tags, talentPoolCaseTag)
	} else {
		if vacancy.Kind == atsdomain.VacancyKindGeneralApplication {
			customFields["ats_candidate_bucket"] = generalApplicationsQueueSlug
		} else {
			customFields["ats_candidate_bucket"] = "job_queue"
		}
		tags = removeTag(tags, talentPoolCaseTag)
	}
	if _, err := host.UpdateCase(ctx, application.CaseID, runtimehost.CaseUpdateInput{
		CustomFields: customFields,
		Tags:         &tags,
	}); err != nil {
		return nil, fmt.Errorf("persist routed candidate case: %w", err)
	}

	noteBody := strings.TrimSpace(input.Note)
	if noteBody == "" {
		noteBody = reason
	}
	if noteBody != "" {
		if _, err := s.store.AddRecruiterNote(ctx, workspaceID, application.ID, firstNonBlank(input.ActorName, "ATS Admin"), actorType(input.ActorType), noteBody); err != nil {
			return nil, err
		}
	}
	return application, nil
}

func (s *Service) updateVacancyState(ctx context.Context, workspaceID, vacancyID string, mutate func(*atsdomain.Vacancy) error) (*Vacancy, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	current, err := s.store.GetVacancy(ctx, workspaceID, vacancyID)
	if err != nil {
		return nil, err
	}
	domainVacancy := current.toDomain()
	if err := mutate(domainVacancy); err != nil {
		return nil, err
	}
	saved, err := s.store.SaveVacancy(ctx, vacancyFromDomain(domainVacancy))
	if err != nil {
		return nil, err
	}
	if err := s.publishCareersSiteIfInstalled(ctx, workspaceID); err != nil {
		return nil, err
	}
	return saved, nil
}

func (s *Service) publishCareersSiteIfInstalled(ctx context.Context, workspaceID string) error {
	return s.publishCareersSite(ctx, workspaceID, true)
}

func (s *Service) publishCareersSite(ctx context.Context, workspaceID string, allowMissing bool) error {
	if s == nil || s.store == nil {
		if allowMissing {
			return nil
		}
		return fmt.Errorf("careers publishing is not configured")
	}
	host, err := s.newHost(ctx)
	if err != nil {
		if allowMissing {
			return nil
		}
		return err
	}
	bundle, err := s.CareersSiteBundle(ctx, workspaceID)
	if err != nil {
		return err
	}
	files, err := renderCareersSite(bundle)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, relativePath := range paths {
		if err := host.PublishArtifact(ctx, runtimehost.PublishArtifactInput{
			Surface:      "website",
			RelativePath: relativePath,
			Content:      files[relativePath],
			ActorID:      "ats-runtime",
		}); err != nil {
			if allowMissing && strings.Contains(strings.ToLower(err.Error()), "not found") {
				return nil
			}
			return fmt.Errorf("publish careers artifact %s: %w", relativePath, err)
		}
	}
	if _, err := s.store.MarkCareersSitePublished(ctx, workspaceID, time.Now().UTC()); err != nil {
		return err
	}
	return nil
}

func (s *Service) resolveVacancyQueue(ctx context.Context, vacancy *Vacancy) (*runtimehost.HostQueue, error) {
	if vacancy == nil {
		return nil, fmt.Errorf("vacancy is required")
	}
	host, err := s.newHost(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(vacancy.CaseQueueID) != "" {
		if queue, found, err := host.GetQueue(ctx, vacancy.CaseQueueID); err == nil && found {
			return queue, nil
		}
	}
	queue, found, err := host.GetQueueBySlug(ctx, vacancy.CaseQueueSlug)
	if err != nil {
		return nil, fmt.Errorf("resolve vacancy queue %s: %w", vacancy.CaseQueueSlug, err)
	}
	if !found {
		return nil, fmt.Errorf("resolve vacancy queue %s: not found", vacancy.CaseQueueSlug)
	}
	return queue, nil
}

func (s *Service) enrichAndFilterCandidateProfiles(ctx context.Context, workspaceID string, profiles []CandidateProfile, options CandidateListOptions) ([]CandidateProfile, error) {
	if len(profiles) == 0 {
		return profiles, nil
	}
	vacancies, err := s.store.ListVacancies(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	vacancyByID := make(map[string]Vacancy, len(vacancies))
	var generalVacancyID string
	for _, vacancy := range vacancies {
		vacancyByID[vacancy.ID] = vacancy
		if vacancy.Kind == atsdomain.VacancyKindGeneralApplication {
			generalVacancyID = vacancy.ID
		}
	}
	stagePresets, err := s.store.ListStagePresets(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	savedViews, err := s.store.ListSavedFilters(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	presetStages := stageSetForSlug(stagePresets, options.StagePresetSlug)
	viewCriteria := savedViewCriteriaForSlug(savedViews, options.ViewSlug)

	talentPoolQueueID := mustQueueID(ctx, s, workspaceID, talentPoolQueueSlug)
	host, err := s.newHost(ctx)
	if err != nil {
		return nil, err
	}
	queueCache := map[string]*runtimehost.HostQueue{}
	filtered := make([]CandidateProfile, 0, len(profiles))
	for _, profile := range profiles {
		if strings.TrimSpace(profile.Application.CaseID) != "" {
			caseObj, found, err := host.GetCase(ctx, profile.Application.CaseID)
			if err == nil && found {
				profile.CaseQueueID = strings.TrimSpace(caseObj.QueueID)
				profile.IsTalentPool = profile.CaseQueueID != "" && profile.CaseQueueID == talentPoolQueueID
				if profile.CaseQueueID != "" {
					if queue, ok := queueCache[profile.CaseQueueID]; ok && queue != nil {
						profile.CaseQueueSlug = queue.Slug
						profile.CaseQueueName = queue.Name
					} else if queue, queueFound, queueErr := host.GetQueue(ctx, profile.CaseQueueID); queueErr == nil && queueFound {
						profile.CaseQueueSlug = queue.Slug
						profile.CaseQueueName = queue.Name
						queueCache[profile.CaseQueueID] = queue
					}
				}
			}
		}

		switch options.Scope {
		case CandidateListScopeGeneral:
			if profile.Application.VacancyID != generalVacancyID {
				continue
			}
		case CandidateListScopeTalentPool:
			if !profile.IsTalentPool {
				continue
			}
		}
		if len(presetStages) > 0 && !presetStages[string(profile.Application.Stage)] {
			continue
		}
		if !matchesSavedViewCriteria(profile, vacancyByID[profile.Application.VacancyID], viewCriteria) {
			continue
		}
		filtered = append(filtered, profile)
	}
	return filtered, nil
}

func filterPrimaryJobs(jobs []Vacancy) []Vacancy {
	filtered := make([]Vacancy, 0, len(jobs))
	for _, job := range jobs {
		if strings.TrimSpace(string(job.Kind)) == "" || job.Kind == atsdomain.VacancyKindJob {
			filtered = append(filtered, job)
		}
	}
	return filtered
}

func appendUniqueTag(tags []string, tag string) []string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return tags
	}
	for _, existing := range tags {
		if strings.EqualFold(strings.TrimSpace(existing), tag) {
			return tags
		}
	}
	return append(tags, tag)
}

func removeTag(tags []string, tag string) []string {
	tag = strings.TrimSpace(strings.ToLower(tag))
	if tag == "" || len(tags) == 0 {
		return tags
	}
	filtered := make([]string, 0, len(tags))
	for _, existing := range tags {
		if strings.TrimSpace(strings.ToLower(existing)) == tag {
			continue
		}
		filtered = append(filtered, existing)
	}
	return filtered
}

func sanitizeMediaFilename(filename string) string {
	filename = strings.TrimSpace(filepath.Base(filename))
	if filename == "" {
		return "asset"
	}
	ext := strings.ToLower(filepath.Ext(filename))
	base := strings.TrimSuffix(filename, ext)
	base = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, base)
	base = strings.Trim(base, "-")
	if base == "" {
		base = "asset"
	}
	if ext == "" {
		ext = ".bin"
	}
	return base + ext
}

func newATSAssetFilename(filename string) string {
	return fmt.Sprintf("%d-%s", time.Now().UTC().UnixNano(), strings.TrimSpace(filename))
}

func mustQueueID(ctx context.Context, s *Service, workspaceID, slug string) string {
	if s == nil {
		return ""
	}
	host, err := s.newHost(ctx)
	if err != nil {
		return ""
	}
	queue, found, err := host.GetQueueBySlug(ctx, slug)
	if err != nil || !found || queue == nil {
		return ""
	}
	return queue.ID
}

func (s *Service) syncSetupStatus(ctx context.Context, workspaceID string, site *CareersSiteProfile, team []CareersTeamMember, gallery []CareersGalleryItem, jobs []Vacancy) (*SetupStatus, error) {
	state, err := s.store.GetCareersSetupState(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.syncSetupStatusWithState(ctx, state, site, team, gallery, jobs)
}

func (s *Service) syncSetupStatusWithState(ctx context.Context, state *CareersSetupState, site *CareersSiteProfile, team []CareersTeamMember, gallery []CareersGalleryItem, jobs []Vacancy) (*SetupStatus, error) {
	if state == nil {
		return nil, fmt.Errorf("careers setup state is required")
	}
	steps := buildSetupChecklist(site, team, gallery, jobs, state.ConfirmedSteps)
	currentStep := strings.TrimSpace(strings.ToLower(state.CurrentStep))
	if currentStep == "" {
		currentStep = firstIncompleteSetupStep(steps)
	}
	if setupStepCompleted(currentStep, steps) {
		currentStep = firstIncompleteSetupStep(steps)
	}

	isCompleted := true
	for _, step := range steps {
		if !step.Completed {
			isCompleted = false
			break
		}
	}

	var completedAt *time.Time
	if isCompleted {
		completedAt = state.CompletedAt
		if completedAt == nil {
			now := time.Now().UTC()
			completedAt = &now
		}
	}

	if currentStep != state.CurrentStep || !timesEqual(completedAt, state.CompletedAt) {
		saved, err := s.store.SaveCareersSetupState(ctx, CareersSetupState{
			WorkspaceID:    state.WorkspaceID,
			CurrentStep:    currentStep,
			ConfirmedSteps: state.ConfirmedSteps,
			CompletedAt:    completedAt,
			CreatedAt:      state.CreatedAt,
		})
		if err != nil {
			return nil, err
		}
		state = saved
	}

	return &SetupStatus{
		WorkspaceID: state.WorkspaceID,
		CurrentStep: state.CurrentStep,
		IsCompleted: isCompleted,
		CompletedAt: state.CompletedAt,
		PublishedAt: site.PublishedAt,
		Steps:       steps,
	}, nil
}

func buildSetupChecklist(site *CareersSiteProfile, team []CareersTeamMember, gallery []CareersGalleryItem, jobs []Vacancy, confirmedSteps []string) []SetupChecklistStep {
	completions := stepCompletions(site, team, gallery, jobs, confirmedSteps)

	return []SetupChecklistStep{
		{
			Key:         "brand",
			Title:       "Brand",
			Description: "Set the company name, site title, colors, and visible brand markers.",
			ActionLabel: "Edit site profile",
			Completed:   completions["brand"],
		},
		{
			Key:         "company",
			Title:       "Company Story",
			Description: "Fill in the hero, company story, contact details, and core public copy.",
			ActionLabel: "Finish company profile",
			Completed:   completions["company"],
		},
		{
			Key:         "team",
			Title:       "Team & Gallery",
			Description: "Show the people and moments that make the public site feel credible.",
			ActionLabel: "Add people and visuals",
			Completed:   completions["team"],
		},
		{
			Key:         "jobs",
			Title:       "Jobs",
			Description: "Create at least one structured job so the generator has something real to publish.",
			ActionLabel: "Create a job",
			Completed:   completions["jobs"],
		},
		{
			Key:         "publish",
			Title:       "Publish",
			Description: "Publish the careers site once the structure and content are in place.",
			ActionLabel: "Publish careers site",
			Completed:   completions["publish"],
		},
	}
}

func stepCompletions(site *CareersSiteProfile, team []CareersTeamMember, gallery []CareersGalleryItem, jobs []Vacancy, confirmedSteps []string) map[string]bool {
	confirmed := normalizedStepSet(confirmedSteps)
	brandReady := siteProfileHasRealBrandContent(site)
	companyReady := siteProfileHasRealCompanyContent(site)
	teamReady := teamHasRealContent(team) && galleryHasRealContent(gallery)
	jobsReady := len(filterPrimaryJobs(jobs)) > 0
	publishReady := site != nil && site.PublishedAt != nil && !site.PublishedAt.IsZero()

	return map[string]bool{
		"brand":   brandReady && confirmed["brand"],
		"company": companyReady && confirmed["company"],
		"team":    teamReady && confirmed["team"],
		"jobs":    jobsReady && confirmed["jobs"],
		"publish": publishReady && confirmed["publish"],
	}
}

func firstIncompleteSetupStep(steps []SetupChecklistStep) string {
	for _, step := range steps {
		if !step.Completed {
			return step.Key
		}
	}
	return "publish"
}

func setupStepCompleted(key string, steps []SetupChecklistStep) bool {
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return false
	}
	for _, step := range steps {
		if step.Key == key {
			return step.Completed
		}
	}
	return false
}

func (s *Service) updateSiteSetupConfirmations(ctx context.Context, workspaceID string, site *CareersSiteProfile) error {
	if err := s.setSetupStepConfirmed(ctx, workspaceID, "brand", siteProfileHasRealBrandContent(site)); err != nil {
		return err
	}
	return s.setSetupStepConfirmed(ctx, workspaceID, "company", siteProfileHasRealCompanyContent(site))
}

func (s *Service) updateTeamSetupConfirmation(ctx context.Context, workspaceID string, team []CareersTeamMember, gallery []CareersGalleryItem) error {
	return s.setSetupStepConfirmed(ctx, workspaceID, "team", teamHasRealContent(team) && galleryHasRealContent(gallery))
}

func (s *Service) setSetupStepConfirmed(ctx context.Context, workspaceID, step string, confirmed bool) error {
	state, err := s.store.GetCareersSetupState(ctx, workspaceID)
	if err != nil {
		return err
	}
	next := updateConfirmedSteps(state.ConfirmedSteps, step, confirmed)
	if stringSlicesEqual([]string(state.ConfirmedSteps), next) {
		return nil
	}
	_, err = s.store.SaveCareersSetupState(ctx, CareersSetupState{
		WorkspaceID:    state.WorkspaceID,
		CurrentStep:    state.CurrentStep,
		ConfirmedSteps: next,
		CompletedAt:    state.CompletedAt,
		CreatedAt:      state.CreatedAt,
	})
	return err
}

func updateConfirmedSteps(existing []string, step string, confirmed bool) []string {
	step = strings.TrimSpace(strings.ToLower(step))
	if step == "" {
		return normalizeSteps(existing)
	}
	normalized := normalizedStepSet(existing)
	if confirmed {
		normalized[step] = true
	} else {
		delete(normalized, step)
	}
	steps := make([]string, 0, len(normalized))
	for _, candidate := range []string{"brand", "company", "team", "jobs", "publish"} {
		if normalized[candidate] {
			steps = append(steps, candidate)
		}
	}
	return steps
}

func normalizedStepSet(steps []string) map[string]bool {
	set := map[string]bool{}
	for _, step := range steps {
		step = strings.TrimSpace(strings.ToLower(step))
		if step == "" {
			continue
		}
		set[step] = true
	}
	return set
}

func normalizeSteps(steps []string) []string {
	normalized := normalizedStepSet(steps)
	ordered := make([]string, 0, len(normalized))
	for _, candidate := range []string{"brand", "company", "team", "jobs", "publish"} {
		if normalized[candidate] {
			ordered = append(ordered, candidate)
		}
	}
	return ordered
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if strings.TrimSpace(left[index]) != strings.TrimSpace(right[index]) {
			return false
		}
	}
	return true
}

func siteProfileHasRealBrandContent(site *CareersSiteProfile) bool {
	if site == nil {
		return false
	}
	seed := defaultCareersSiteProfile(site.WorkspaceID)
	required := strings.TrimSpace(site.CompanyName) != "" &&
		strings.TrimSpace(site.SiteTitle) != "" &&
		strings.TrimSpace(site.PrimaryColor) != ""
	if !required {
		return false
	}
	return strings.TrimSpace(site.CompanyName) != strings.TrimSpace(seed.CompanyName) ||
		strings.TrimSpace(site.SiteTitle) != strings.TrimSpace(seed.SiteTitle) ||
		strings.TrimSpace(site.PrimaryColor) != strings.TrimSpace(seed.PrimaryColor) ||
		strings.TrimSpace(site.LogoURL) != "" ||
		strings.TrimSpace(site.HeroImageURL) != "" ||
		strings.TrimSpace(site.OgImageURL) != ""
}

func siteProfileHasRealCompanyContent(site *CareersSiteProfile) bool {
	if site == nil {
		return false
	}
	seed := defaultCareersSiteProfile(site.WorkspaceID)
	required := strings.TrimSpace(site.Tagline) != "" &&
		strings.TrimSpace(site.HeroTitle) != "" &&
		strings.TrimSpace(site.HeroBody) != "" &&
		strings.TrimSpace(site.StoryBody) != "" &&
		strings.TrimSpace(site.ContactEmail) != ""
	if !required {
		return false
	}
	return strings.TrimSpace(site.Tagline) != strings.TrimSpace(seed.Tagline) ||
		strings.TrimSpace(site.HeroTitle) != strings.TrimSpace(seed.HeroTitle) ||
		strings.TrimSpace(site.HeroBody) != strings.TrimSpace(seed.HeroBody) ||
		strings.TrimSpace(site.StoryBody) != strings.TrimSpace(seed.StoryBody) ||
		strings.TrimSpace(site.ContactEmail) != strings.TrimSpace(seed.ContactEmail) ||
		strings.TrimSpace(site.WebsiteURL) != strings.TrimSpace(seed.WebsiteURL)
}

func teamHasRealContent(team []CareersTeamMember) bool {
	if len(team) == 0 {
		return false
	}
	firstWorkspaceID := ""
	for _, member := range team {
		if strings.TrimSpace(member.WorkspaceID) != "" {
			firstWorkspaceID = member.WorkspaceID
			break
		}
	}
	defaults := defaultCareersTeamMembers(firstWorkspaceID)
	return !sameTeamContent(team, defaults)
}

func galleryHasRealContent(gallery []CareersGalleryItem) bool {
	if len(gallery) == 0 {
		return false
	}
	firstWorkspaceID := ""
	for _, item := range gallery {
		if strings.TrimSpace(item.WorkspaceID) != "" {
			firstWorkspaceID = item.WorkspaceID
			break
		}
	}
	defaults := defaultCareersGalleryItems(firstWorkspaceID)
	return !sameGalleryContent(gallery, defaults)
}

func sameTeamContent(left, right []CareersTeamMember) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if normalizeCareersTeamMember(left[index]).Name != normalizeCareersTeamMember(right[index]).Name ||
			normalizeCareersTeamMember(left[index]).Role != normalizeCareersTeamMember(right[index]).Role ||
			normalizeCareersTeamMember(left[index]).Bio != normalizeCareersTeamMember(right[index]).Bio ||
			strings.TrimSpace(left[index].ImageURL) != strings.TrimSpace(right[index].ImageURL) ||
			strings.TrimSpace(left[index].LinkedInURL) != strings.TrimSpace(right[index].LinkedInURL) {
			return false
		}
	}
	return true
}

func sameGalleryContent(left, right []CareersGalleryItem) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		leftItem := normalizeCareersGalleryItem(left[index])
		rightItem := normalizeCareersGalleryItem(right[index])
		if leftItem.Section != rightItem.Section ||
			leftItem.AltText != rightItem.AltText ||
			leftItem.Caption != rightItem.Caption ||
			leftItem.ImageURL != rightItem.ImageURL {
			return false
		}
	}
	return true
}

func stageSetForSlug(presets []StagePreset, slug string) map[string]bool {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if slug == "" {
		return nil
	}
	for _, preset := range presets {
		if strings.TrimSpace(strings.ToLower(preset.Slug)) != slug {
			continue
		}
		set := map[string]bool{}
		for _, stage := range preset.Stages {
			stage = strings.TrimSpace(strings.ToLower(stage))
			if stage != "" {
				set[stage] = true
			}
		}
		return set
	}
	return nil
}

func savedViewCriteriaForSlug(filters []SavedFilter, slug string) SavedViewCriteria {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if slug == "" {
		return SavedViewCriteria{}
	}
	for _, filter := range filters {
		if strings.TrimSpace(strings.ToLower(filter.Slug)) != slug {
			continue
		}
		var criteria SavedViewCriteria
		if err := json.Unmarshal(filter.Criteria, &criteria); err == nil {
			criteria.Stages = normalizeStringList(criteria.Stages)
			criteria.SourceKinds = normalizeStringList(criteria.SourceKinds)
			criteria.QueueSlugs = normalizeStringList(criteria.QueueSlugs)
			criteria.VacancyStatuses = normalizeStringList(criteria.VacancyStatuses)
			criteria.VacancyKinds = normalizeStringList(criteria.VacancyKinds)
			return criteria
		}
	}
	return SavedViewCriteria{}
}

func matchesSavedViewCriteria(profile CandidateProfile, vacancy Vacancy, criteria SavedViewCriteria) bool {
	if len(criteria.Stages) > 0 && !containsNormalized(criteria.Stages, string(profile.Application.Stage)) {
		return false
	}
	if len(criteria.SourceKinds) > 0 && !containsNormalized(criteria.SourceKinds, string(profile.Application.SourceKind)) {
		return false
	}
	if len(criteria.QueueSlugs) > 0 && !containsNormalized(criteria.QueueSlugs, profile.CaseQueueSlug) {
		return false
	}
	if len(criteria.VacancyStatuses) > 0 && !containsNormalized(criteria.VacancyStatuses, string(vacancy.Status)) {
		return false
	}
	if len(criteria.VacancyKinds) > 0 && !containsNormalized(criteria.VacancyKinds, string(vacancy.Kind)) {
		return false
	}
	if criteria.TalentPoolOnly && !profile.IsTalentPool {
		return false
	}
	return true
}

func containsNormalized(values []string, target string) bool {
	target = strings.TrimSpace(strings.ToLower(target))
	for _, value := range values {
		if strings.TrimSpace(strings.ToLower(value)) == target {
			return true
		}
	}
	return false
}

func timesEqual(left, right *time.Time) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return left.UTC().Equal(right.UTC())
	}
}

func occurredAt(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func actorType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "recruiter"
	}
	return value
}
