package atsruntime

import (
	"context"
	"fmt"
	"strings"
	"time"

	atsdomain "github.com/movebigrocks/platform/extensions/ats/runtime/domain"
	automationservices "github.com/movebigrocks/platform/internal/automation/services"
	platformsql "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

type Service struct {
	platformStore *platformsql.Store
	store         *Store
	queueService  *serviceapp.QueueService
	contact       *platformservices.ContactService
	cases         *serviceapp.CaseService
	rules         *automationservices.RulesEngine
}

func NewService(
	platformStore *platformsql.Store,
	store *Store,
	queueService *serviceapp.QueueService,
	contact *platformservices.ContactService,
	caseService *serviceapp.CaseService,
	rules *automationservices.RulesEngine,
) *Service {
	return &Service{
		platformStore: platformStore,
		store:         store,
		queueService:  queueService,
		contact:       contact,
		cases:         caseService,
		rules:         rules,
	}
}

func (s *Service) CreateJob(ctx context.Context, input CreateJobInput) (*Vacancy, error) {
	if s == nil || s.platformStore == nil || s.store == nil || s.queueService == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}

	var created *Vacancy
	err := s.platformStore.WithTransaction(ctx, func(txCtx context.Context) error {
		if _, err := s.store.EnsureWorkspaceDefaults(txCtx, input.WorkspaceID); err != nil {
			return err
		}
		vacancy, err := s.store.CreateVacancy(txCtx, input)
		if err != nil {
			return err
		}
		if _, err := s.queueService.CreateQueue(txCtx, serviceapp.CreateQueueParams{
			WorkspaceID: vacancy.WorkspaceID,
			Name:        vacancy.Title + " Candidates",
			Slug:        vacancy.CaseQueueSlug,
			Description: "Candidate review queue for " + vacancy.Title,
		}); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return fmt.Errorf("create vacancy queue: %w", err)
		}
		created = vacancy
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *Service) ListJobs(ctx context.Context, workspaceID string) ([]Vacancy, error) {
	if _, err := s.store.EnsureWorkspaceDefaults(ctx, workspaceID); err != nil {
		return nil, err
	}
	return s.store.ListVacancies(ctx, workspaceID)
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

func (s *Service) ListCandidates(ctx context.Context, workspaceID, vacancyID string) ([]CandidateProfile, error) {
	return s.store.ListCandidateProfiles(ctx, workspaceID, vacancyID)
}

func (s *Service) WorkspaceDefaults(ctx context.Context, workspaceID string) (*WorkspaceDefaults, error) {
	return s.store.EnsureWorkspaceDefaults(ctx, workspaceID)
}

func (s *Service) SubmitApplication(ctx context.Context, input SubmitApplicationInput) (*SubmissionResult, error) {
	if s == nil || s.platformStore == nil || s.store == nil || s.contact == nil || s.cases == nil {
		return nil, fmt.Errorf("ats service is not configured")
	}
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return nil, fmt.Errorf("workspace ID is required")
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

		queue, err := s.platformStore.Queues().GetQueueBySlug(txCtx, vacancy.WorkspaceID, vacancy.CaseQueueSlug)
		if err != nil {
			return fmt.Errorf("resolve vacancy queue %s: %w", vacancy.CaseQueueSlug, err)
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

		caseObj, err := s.cases.CreateCase(txCtx, serviceapp.CreateCaseParams{
			WorkspaceID:  vacancy.WorkspaceID,
			Subject:      fmt.Sprintf("%s for %s", applicant.FullName, vacancy.Title),
			Description:  applicant.CoverNote,
			QueueID:      queue.ID,
			ContactID:    contact.ID,
			ContactName:  applicant.FullName,
			ContactEmail: applicant.Email,
			Category:     "recruiting",
			Tags: []string{
				"ats",
				"candidate",
				"applied",
				vacancy.Slug,
			},
			CustomFields: customFields,
		})
		if err != nil {
			return fmt.Errorf("create candidate case: %w", err)
		}
		if err := s.linkSubmissionAttachments(txCtx, vacancy.WorkspaceID, caseObj.ID, applicant); err != nil {
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

func (s *Service) linkSubmissionAttachments(ctx context.Context, workspaceID, caseID string, applicant *Applicant) error {
	if s == nil || s.platformStore == nil || applicant == nil {
		return nil
	}

	resumeAttachmentID := strings.TrimSpace(applicant.ResumeAttachmentID)
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
			if err := s.rules.EvaluateRulesForCaseTyped(txCtx, caseObj, "ats_application_stage_changed", changes); err != nil {
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
	return s.store.SaveVacancy(ctx, vacancyFromDomain(domainVacancy))
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
