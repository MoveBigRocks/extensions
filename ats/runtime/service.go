package atsruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	atsdomain "github.com/movebigrocks/platform/extensions/ats/runtime/domain"
	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	sharedstore "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformsql "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

type attachmentUploader interface {
	Upload(ctx context.Context, attachment *servicedomain.Attachment, reader io.Reader) error
}

type Service struct {
	platformStore *platformsql.Store
	store         *Store
	queueService  *serviceapp.QueueService
	contact       *platformservices.ContactService
	cases         *serviceapp.CaseService
	rules         *automationservices.RulesEngine
	extension     *platformservices.ExtensionService
	attachments   attachmentUploader
}

const (
	generalApplicationsQueueSlug  = "general-applications"
	talentPoolQueueSlug           = "talent-pool"
	generalApplicationVacancySlug = "general-application"
	talentPoolCaseTag             = "ats-talent-pool"
)

func NewService(
	platformStore *platformsql.Store,
	store *Store,
	queueService *serviceapp.QueueService,
	contact *platformservices.ContactService,
	caseService *serviceapp.CaseService,
	rules *automationservices.RulesEngine,
	extensionService *platformservices.ExtensionService,
	attachments attachmentUploader,
) *Service {
	return &Service{
		platformStore: platformStore,
		store:         store,
		queueService:  queueService,
		contact:       contact,
		cases:         caseService,
		rules:         rules,
		extension:     extensionService,
		attachments:   attachments,
	}
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

func (s *Service) ensureRoutingQueue(ctx context.Context, workspaceID, slug, name, description string) (*servicedomain.Queue, error) {
	if s == nil || s.queueService == nil || s.platformStore == nil {
		return nil, fmt.Errorf("queue service is not configured")
	}
	if queue, err := s.platformStore.Queues().GetQueueBySlug(ctx, workspaceID, slug); err == nil {
		return queue, nil
	} else if err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
		return nil, fmt.Errorf("load queue %s: %w", slug, err)
	}
	queue, err := s.queueService.CreateQueue(ctx, serviceapp.CreateQueueParams{
		WorkspaceID: strings.TrimSpace(workspaceID),
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
	queue, err = s.platformStore.Queues().GetQueueBySlug(ctx, workspaceID, slug)
	if err != nil {
		return nil, fmt.Errorf("load queue %s: %w", slug, err)
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
	if s == nil || s.platformStore == nil || s.store == nil || s.queueService == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}

	var created *Vacancy
	err := s.platformStore.WithTransaction(ctx, func(txCtx context.Context) error {
		if _, err := s.ensureWorkspaceProvisioned(txCtx, input.WorkspaceID); err != nil {
			return err
		}
		vacancy, err := s.store.CreateVacancy(txCtx, input)
		if err != nil {
			return err
		}
		queue, err := s.queueService.CreateQueue(txCtx, serviceapp.CreateQueueParams{
			WorkspaceID: vacancy.WorkspaceID,
			Name:        vacancy.Title + " Candidates",
			Slug:        vacancy.CaseQueueSlug,
			Description: "Candidate review queue for " + vacancy.Title,
		})
		if err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				return fmt.Errorf("create vacancy queue: %w", err)
			}
			queue, err = s.platformStore.Queues().GetQueueBySlug(txCtx, vacancy.WorkspaceID, vacancy.CaseQueueSlug)
			if err != nil {
				return fmt.Errorf("load existing vacancy queue: %w", err)
			}
		}
		vacancy.CaseQueueID = queue.ID
		vacancy, err = s.store.SaveVacancy(txCtx, vacancy)
		if err != nil {
			return err
		}
		created = vacancy
		return nil
	})
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
		ResumeUploadsEnabled: s.attachments != nil,
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
	if s == nil || s.platformStore == nil || s.store == nil || s.contact == nil || s.cases == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if _, err := s.ensureWorkspaceProvisioned(ctx, input.WorkspaceID); err != nil {
		return nil, err
	}

	var result *SubmissionResult
	err := s.platformStore.WithTransaction(ctx, func(txCtx context.Context) error {
		vacancy, err := s.store.GetVacancyBySlug(txCtx, input.WorkspaceID, input.VacancySlug)
		if err != nil {
			return err
		}
		applicantDomain, applicationDomain, err := atsdomain.BuildCandidateRecord(input.WorkspaceID, vacancy.toDomain(), input.Submission)
		if err != nil {
			return err
		}

		contact, err := s.contact.CreateContact(txCtx, platformservices.CreateContactParams{
			WorkspaceID: input.WorkspaceID,
			Email:       applicantDomain.Email,
			Name:        applicantDomain.FullName,
			Phone:       applicantDomain.Phone,
			Source:      "ats",
			Metadata: map[string]interface{}{
				"ats_vacancy_slug": vacancy.Slug,
			},
		})
		if err != nil {
			return fmt.Errorf("create applicant contact: %w", err)
		}

		applicant := applicantFromDomain(applicantDomain)
		applicant.ContactID = contact.ID
		applicant, err = s.store.UpsertApplicant(txCtx, applicant)
		if err != nil {
			return err
		}

		application := applicationFromDomain(applicationDomain)
		application.ApplicantID = applicant.ID
		application.ContactID = contact.ID

		queue, err := s.resolveVacancyQueue(txCtx, vacancy)
		if err != nil {
			return err
		}

		customFields := shareddomain.NewTypedCustomFields()
		for key, value := range vacancy.toDomain().CaseCustomFields() {
			customFields.SetAny(key, value)
		}
		for key, value := range applicantDomain.CaseCustomFields() {
			customFields.SetAny(key, value)
		}
		for key, value := range applicationDomain.CaseCustomFields() {
			customFields.SetAny(key, value)
		}
		customFields.SetString("ats_case_queue_id", queue.ID)
		customFields.SetString("ats_case_queue_slug", queue.Slug)
		if vacancy.Kind == atsdomain.VacancyKindGeneralApplication {
			customFields.SetString("ats_candidate_bucket", generalApplicationsQueueSlug)
		} else {
			customFields.SetString("ats_candidate_bucket", "job_queue")
		}

		tags := []string{
			"ats",
			"candidate",
			"application",
			"applied",
		}
		if vacancy.Kind == atsdomain.VacancyKindGeneralApplication {
			tags = append(tags, "general-application")
		} else {
			tags = append(tags, "job:"+vacancy.Slug)
		}

		caseObj, err := s.cases.CreateCase(txCtx, serviceapp.CreateCaseParams{
			WorkspaceID:  vacancy.WorkspaceID,
			Subject:      fmt.Sprintf("%s for %s", applicant.FullName, vacancy.Title),
			Description:  application.SubmissionCoverNote,
			QueueID:      queue.ID,
			ContactID:    contact.ID,
			ContactName:  applicant.FullName,
			ContactEmail: applicant.Email,
			Category:     "recruiting",
			Tags:         tags,
			CustomFields: customFields,
		})
		if err != nil {
			return fmt.Errorf("create candidate case: %w", err)
		}
		if err := s.linkSubmissionAttachments(txCtx, vacancy.WorkspaceID, caseObj.ID, application); err != nil {
			return err
		}

		application.CaseID = caseObj.ID
		savedApplication, err := s.store.CreateApplication(txCtx, application)
		if err != nil {
			return err
		}

		result = &SubmissionResult{
			Vacancy:     *vacancy,
			Applicant:   *applicant,
			Application: *savedApplication,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) UploadCareerAttachment(ctx context.Context, workspaceID, filename, contentType, description string, size int64, reader io.Reader) (*servicedomain.Attachment, error) {
	if s == nil || s.platformStore == nil || s.attachments == nil {
		return nil, fmt.Errorf("resume uploads are not configured")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	attachment := servicedomain.NewAttachment(
		workspaceID,
		strings.TrimSpace(filename),
		strings.TrimSpace(contentType),
		size,
		servicedomain.AttachmentSourceUpload,
	)
	attachment.Description = strings.TrimSpace(description)
	if err := s.attachments.Upload(ctx, attachment, reader); err != nil {
		return nil, err
	}
	if err := s.platformStore.Cases().SaveAttachment(ctx, attachment, nil); err != nil {
		return nil, fmt.Errorf("save attachment metadata: %w", err)
	}
	return attachment, nil
}

func (s *Service) UploadCareersMediaAsset(ctx context.Context, workspaceID, purpose, filename, contentType string, size int64, reader io.Reader) (*CareersMediaAsset, error) {
	if s == nil || s.extension == nil || s.platformStore == nil {
		return nil, fmt.Errorf("careers media publishing is not configured")
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
	installed, err := s.platformStore.Extensions().GetInstalledExtensionBySlug(ctx, workspaceID, "ats")
	if err != nil {
		return nil, fmt.Errorf("resolve installed ats extension: %w", err)
	}
	safeName := sanitizeMediaFilename(filename)
	assetID := "media_" + strings.ReplaceAll(strings.TrimSuffix(safeName, filepath.Ext(safeName)), ".", "-")
	if len(assetID) > 64 {
		assetID = assetID[:64]
	}
	assetPath := path.Join("site/assets/uploads", newATSAssetFilename(safeName))
	if _, err := s.extension.PublishExtensionArtifact(ctx, installed.ID, "website", assetPath, payload, "ats-runtime"); err != nil {
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

func (s *Service) linkSubmissionAttachments(ctx context.Context, workspaceID, caseID string, application *Application) error {
	if s == nil || s.platformStore == nil || application == nil {
		return nil
	}

	resumeAttachmentID := strings.TrimSpace(application.SubmissionResumeAttachmentID)
	if resumeAttachmentID == "" {
		return nil
	}

	attachment, err := s.platformStore.Cases().GetAttachment(ctx, workspaceID, resumeAttachmentID)
	if err != nil {
		return fmt.Errorf("load resume attachment %s: %w", resumeAttachmentID, err)
	}
	if attachment.Status != servicedomain.AttachmentStatusClean {
		return fmt.Errorf("resume attachment %s is not ready for ATS intake", attachment.ID)
	}
	if strings.TrimSpace(attachment.CaseID) != "" && strings.TrimSpace(attachment.CaseID) != caseID {
		return fmt.Errorf("resume attachment %s is already linked to case %s", attachment.ID, attachment.CaseID)
	}
	if err := s.platformStore.Cases().LinkAttachmentsToCase(ctx, workspaceID, caseID, []string{attachment.ID}); err != nil {
		return fmt.Errorf("link resume attachment %s to case %s: %w", attachment.ID, caseID, err)
	}
	return nil
}

func (s *Service) AddRecruiterNote(ctx context.Context, workspaceID, applicationID, body, authorName, authorType string) (*RecruiterNote, error) {
	return s.store.AddRecruiterNote(ctx, workspaceID, applicationID, authorName, authorType, body)
}

func (s *Service) ChangeCandidateStage(ctx context.Context, workspaceID, applicationID string, input StageChangeInput) (*Application, error) {
	if s == nil || s.platformStore == nil || s.store == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(applicationID) == "" {
		return nil, fmt.Errorf("workspace ID and application ID are required")
	}

	var saved *Application
	err := s.platformStore.WithTransaction(ctx, func(txCtx context.Context) error {
		current, err := s.store.GetApplication(txCtx, workspaceID, applicationID)
		if err != nil {
			return err
		}
		previousStage := current.Stage
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
		if strings.TrimSpace(saved.CaseID) != "" {
			caseObj, err := s.platformStore.Cases().GetCase(txCtx, saved.CaseID)
			if err != nil {
				return fmt.Errorf("load candidate case: %w", err)
			}
			caseObj.CustomFields.SetString("ats_application_stage", string(saved.Stage))
			if saved.RejectionReason != "" {
				caseObj.CustomFields.SetString("ats_application_rejection_reason", saved.RejectionReason)
			}
			if err := s.platformStore.Cases().UpdateCase(txCtx, caseObj); err != nil {
				return fmt.Errorf("update candidate case stage mirror: %w", err)
			}
			if s.rules == nil {
				return nil
			}
			changes := automationservices.NewFieldChanges()
			changes.SetString("ats_application_previous_stage", string(previousStage))
			changes.SetString("ats_application_stage", string(saved.Stage))
			if err := s.rules.EvaluateRulesForCase(txCtx, caseObj, "ats_application_stage_changed", changes); err != nil {
				return fmt.Errorf("evaluate ats stage-change rules: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

func (s *Service) RouteCandidate(ctx context.Context, workspaceID, applicationID string, input CandidateRouteInput) (*Application, error) {
	if s == nil || s.platformStore == nil || s.store == nil || s.cases == nil {
		return nil, fmt.Errorf("ats service is not configured")
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

	var targetQueue *servicedomain.Queue
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

	if err := s.cases.HandoffCase(ctx, application.CaseID, serviceapp.CaseHandoffParams{
		QueueID:         targetQueue.ID,
		Reason:          reason,
		PerformedByName: strings.TrimSpace(input.ActorName),
		PerformedByType: actorType(input.ActorType),
	}); err != nil {
		return nil, fmt.Errorf("route candidate case: %w", err)
	}

	caseObj, err := s.platformStore.Cases().GetCase(ctx, application.CaseID)
	if err != nil {
		return nil, fmt.Errorf("load routed candidate case: %w", err)
	}
	caseObj.CustomFields.SetString("ats_case_queue_id", targetQueue.ID)
	caseObj.CustomFields.SetString("ats_case_queue_slug", targetQueue.Slug)
	if destination == string(CandidateListScopeTalentPool) {
		caseObj.CustomFields.SetString("ats_candidate_bucket", talentPoolQueueSlug)
		caseObj.Tags = appendUniqueTag(caseObj.Tags, talentPoolCaseTag)
	} else {
		if vacancy.Kind == atsdomain.VacancyKindGeneralApplication {
			caseObj.CustomFields.SetString("ats_candidate_bucket", generalApplicationsQueueSlug)
		} else {
			caseObj.CustomFields.SetString("ats_candidate_bucket", "job_queue")
		}
		caseObj.Tags = removeTag(caseObj.Tags, talentPoolCaseTag)
	}
	if err := s.platformStore.Cases().UpdateCase(ctx, caseObj); err != nil {
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
	if s == nil || s.extension == nil || s.platformStore == nil {
		if allowMissing {
			return nil
		}
		return fmt.Errorf("careers publishing is not configured")
	}
	bundle, err := s.CareersSiteBundle(ctx, workspaceID)
	if err != nil {
		return err
	}
	files, err := renderCareersSite(bundle)
	if err != nil {
		return err
	}
	installed, err := s.platformStore.Extensions().GetInstalledExtensionBySlug(ctx, workspaceID, "ats")
	if err != nil {
		if allowMissing && errors.Is(err, sharedstore.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("resolve installed ats extension: %w", err)
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, relativePath := range paths {
		if _, err := s.extension.PublishExtensionArtifact(ctx, installed.ID, "website", relativePath, files[relativePath], "ats-runtime"); err != nil {
			if allowMissing && strings.Contains(strings.ToLower(err.Error()), "artifact service not configured") {
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

func (s *Service) resolveVacancyQueue(ctx context.Context, vacancy *Vacancy) (*servicedomain.Queue, error) {
	if vacancy == nil {
		return nil, fmt.Errorf("vacancy is required")
	}
	if strings.TrimSpace(vacancy.CaseQueueID) != "" {
		queue, err := s.platformStore.Queues().GetQueue(ctx, vacancy.CaseQueueID)
		if err == nil {
			return queue, nil
		}
	}
	queue, err := s.platformStore.Queues().GetQueueBySlug(ctx, vacancy.WorkspaceID, vacancy.CaseQueueSlug)
	if err != nil {
		return nil, fmt.Errorf("resolve vacancy queue %s: %w", vacancy.CaseQueueSlug, err)
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
	queueCache := map[string]*servicedomain.Queue{}
	filtered := make([]CandidateProfile, 0, len(profiles))
	for _, profile := range profiles {
		if strings.TrimSpace(profile.Application.CaseID) != "" {
			caseObj, err := s.platformStore.Cases().GetCase(ctx, profile.Application.CaseID)
			if err == nil {
				profile.CaseQueueID = strings.TrimSpace(caseObj.QueueID)
				profile.IsTalentPool = profile.CaseQueueID != "" && profile.CaseQueueID == talentPoolQueueID
				if profile.CaseQueueID != "" {
					if queue, ok := queueCache[profile.CaseQueueID]; ok && queue != nil {
						profile.CaseQueueSlug = queue.Slug
						profile.CaseQueueName = queue.Name
					} else if queue, queueErr := s.platformStore.Queues().GetQueue(ctx, profile.CaseQueueID); queueErr == nil {
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
	if s == nil || s.platformStore == nil {
		return ""
	}
	queue, err := s.platformStore.Queues().GetQueueBySlug(ctx, workspaceID, slug)
	if err != nil {
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
