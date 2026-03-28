package atsruntime

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	atsdomain "github.com/movebigrocks/platform/extensions/ats/runtime/domain"
	platformsql "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
)

type Store struct {
	db *platformsql.SqlxDB
}

func NewStore(db *platformsql.SqlxDB) (*Store, error) {
	store := &Store{db: db}
	if err := store.ensureSchemaAvailable(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) ensureSchemaAvailable(ctx context.Context) error {
	var regclass sql.NullString
	if err := s.db.Get(ctx).GetContext(ctx, &regclass, s.query(`SELECT to_regclass(?)`), SchemaName+".vacancies"); err != nil {
		return fmt.Errorf("check ats schema availability: %w", err)
	}
	if !regclass.Valid || strings.TrimSpace(regclass.String) == "" {
		return fmt.Errorf("ats schema %s is not available", SchemaName)
	}
	return nil
}

func (s *Store) EnsureWorkspaceDefaults(ctx context.Context, workspaceID string) (*WorkspaceDefaults, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if err := s.db.Transaction(ctx, func(txCtx context.Context) error {
		if err := s.ensureStagePresets(txCtx, workspaceID); err != nil {
			return err
		}
		return s.ensureSavedFilters(txCtx, workspaceID)
	}); err != nil {
		return nil, err
	}

	stagePresets, err := s.ListStagePresets(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	savedFilters, err := s.ListSavedFilters(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return &WorkspaceDefaults{
		StagePresets: stagePresets,
		SavedFilters: savedFilters,
	}, nil
}

func (s *Store) CreateVacancy(ctx context.Context, input CreateJobInput) (*Vacancy, error) {
	vacancyDomain, err := atsdomain.NewVacancy(input.WorkspaceID, input.Slug, input.Title)
	if err != nil {
		return nil, err
	}
	vacancyDomain.Team = strings.TrimSpace(input.Team)
	vacancyDomain.Location = strings.TrimSpace(input.Location)
	if input.WorkMode != "" {
		vacancyDomain.WorkMode = input.WorkMode
	}
	if input.EmploymentType != "" {
		vacancyDomain.EmploymentType = input.EmploymentType
	}
	vacancyDomain.Summary = strings.TrimSpace(input.Summary)
	vacancyDomain.Description = strings.TrimSpace(input.Description)
	vacancyDomain.CaseQueueSlug = vacancyDomain.Slug + "-candidates"
	if err := vacancyDomain.Validate(); err != nil {
		return nil, err
	}

	vacancy := vacancyFromDomain(vacancyDomain)
	if err := s.db.Get(ctx).GetContext(ctx, vacancy, s.query(`
		INSERT INTO ${SCHEMA_NAME}.vacancies (
			id, workspace_id, slug, title, team, location, work_mode, employment_type,
			status, summary, description, application_form_slug, case_queue_slug, careers_path,
			published_at, closed_at, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, workspace_id, slug, title, team, location, work_mode, employment_type,
			status, summary, description, application_form_slug, case_queue_slug, careers_path,
			published_at, closed_at, created_at, updated_at
	`),
		vacancy.ID,
		vacancy.WorkspaceID,
		vacancy.Slug,
		vacancy.Title,
		vacancy.Team,
		vacancy.Location,
		string(vacancy.WorkMode),
		string(vacancy.EmploymentType),
		string(vacancy.Status),
		vacancy.Summary,
		vacancy.Description,
		vacancy.ApplicationFormSlug,
		vacancy.CaseQueueSlug,
		vacancy.CareersPath,
		vacancy.PublishedAt,
		vacancy.ClosedAt,
		vacancy.CreatedAt,
		vacancy.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("create vacancy: %w", err)
	}
	return vacancy, nil
}

func (s *Store) ListVacancies(ctx context.Context, workspaceID string) ([]Vacancy, error) {
	var vacancies []Vacancy
	if err := s.db.Get(ctx).SelectContext(ctx, &vacancies, s.query(`
		SELECT id, workspace_id, slug, title, team, location, work_mode, employment_type,
			status, summary, description, application_form_slug, case_queue_slug, careers_path,
			published_at, closed_at, created_at, updated_at
		FROM ${SCHEMA_NAME}.vacancies
		WHERE workspace_id = ?
		ORDER BY created_at DESC
	`), workspaceID); err != nil {
		return nil, fmt.Errorf("list vacancies: %w", err)
	}
	return vacancies, nil
}

func (s *Store) GetVacancy(ctx context.Context, workspaceID, vacancyID string) (*Vacancy, error) {
	vacancy := &Vacancy{}
	if err := s.db.Get(ctx).GetContext(ctx, vacancy, s.query(`
		SELECT id, workspace_id, slug, title, team, location, work_mode, employment_type,
			status, summary, description, application_form_slug, case_queue_slug, careers_path,
			published_at, closed_at, created_at, updated_at
		FROM ${SCHEMA_NAME}.vacancies
		WHERE workspace_id = ? AND id = ?
	`), workspaceID, vacancyID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("vacancy not found")
		}
		return nil, fmt.Errorf("get vacancy: %w", err)
	}
	return vacancy, nil
}

func (s *Store) GetVacancyBySlug(ctx context.Context, workspaceID, slug string) (*Vacancy, error) {
	vacancy := &Vacancy{}
	if err := s.db.Get(ctx).GetContext(ctx, vacancy, s.query(`
		SELECT id, workspace_id, slug, title, team, location, work_mode, employment_type,
			status, summary, description, application_form_slug, case_queue_slug, careers_path,
			published_at, closed_at, created_at, updated_at
		FROM ${SCHEMA_NAME}.vacancies
		WHERE workspace_id = ? AND slug = ?
	`), workspaceID, strings.TrimSpace(slug)); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("vacancy not found")
		}
		return nil, fmt.Errorf("get vacancy by slug: %w", err)
	}
	return vacancy, nil
}

func (s *Store) SaveVacancy(ctx context.Context, vacancy *Vacancy) (*Vacancy, error) {
	if vacancy == nil {
		return nil, fmt.Errorf("vacancy is required")
	}
	saved := &Vacancy{}
	if err := s.db.Get(ctx).GetContext(ctx, saved, s.query(`
		UPDATE ${SCHEMA_NAME}.vacancies
		SET status = ?, summary = ?, description = ?, application_form_slug = ?, case_queue_slug = ?,
			careers_path = ?, published_at = ?, closed_at = ?, updated_at = ?
		WHERE workspace_id = ? AND id = ?
		RETURNING id, workspace_id, slug, title, team, location, work_mode, employment_type,
			status, summary, description, application_form_slug, case_queue_slug, careers_path,
			published_at, closed_at, created_at, updated_at
	`),
		string(vacancy.Status),
		vacancy.Summary,
		vacancy.Description,
		vacancy.ApplicationFormSlug,
		vacancy.CaseQueueSlug,
		vacancy.CareersPath,
		vacancy.PublishedAt,
		vacancy.ClosedAt,
		vacancy.UpdatedAt,
		vacancy.WorkspaceID,
		vacancy.ID,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("vacancy not found")
		}
		return nil, fmt.Errorf("save vacancy: %w", err)
	}
	return saved, nil
}

func (s *Store) UpsertApplicant(ctx context.Context, applicant *Applicant) (*Applicant, error) {
	if applicant == nil {
		return nil, fmt.Errorf("applicant is required")
	}
	now := time.Now().UTC()
	if applicant.CreatedAt.IsZero() {
		applicant.CreatedAt = now
	}
	applicant.UpdatedAt = now

	var existing Applicant
	err := s.db.Get(ctx).GetContext(ctx, &existing, s.query(`
		SELECT id, workspace_id, contact_id, full_name, email, phone, location, linkedin_url,
			portfolio_url, cover_note, resume_attachment_id, cover_letter_attachment, created_at, updated_at
		FROM ${SCHEMA_NAME}.applicants
		WHERE workspace_id = ? AND email = ?
	`), applicant.WorkspaceID, applicant.Email)
	switch {
	case err == nil:
		applicant.ID = existing.ID
		if applicant.ContactID == "" {
			applicant.ContactID = existing.ContactID
		}
		saved := &Applicant{}
		if err := s.db.Get(ctx).GetContext(ctx, saved, s.query(`
			UPDATE ${SCHEMA_NAME}.applicants
			SET contact_id = ?, full_name = ?, phone = ?, location = ?, linkedin_url = ?,
				portfolio_url = ?, cover_note = ?, resume_attachment_id = ?, cover_letter_attachment = ?, updated_at = ?
			WHERE workspace_id = ? AND id = ?
			RETURNING id, workspace_id, contact_id, full_name, email, phone, location, linkedin_url,
				portfolio_url, cover_note, resume_attachment_id, cover_letter_attachment, created_at, updated_at
		`),
			applicant.ContactID,
			applicant.FullName,
			applicant.Phone,
			applicant.Location,
			applicant.LinkedInURL,
			applicant.PortfolioURL,
			applicant.CoverNote,
			applicant.ResumeAttachmentID,
			applicant.CoverLetterAttachment,
			applicant.UpdatedAt,
			applicant.WorkspaceID,
			applicant.ID,
		); err != nil {
			return nil, fmt.Errorf("update applicant: %w", err)
		}
		return saved, nil
	case err != nil && err != sql.ErrNoRows:
		return nil, fmt.Errorf("lookup applicant: %w", err)
	}

	if applicant.ID == "" {
		applicant.ID = uuid.NewString()
	}
	saved := &Applicant{}
	if err := s.db.Get(ctx).GetContext(ctx, saved, s.query(`
		INSERT INTO ${SCHEMA_NAME}.applicants (
			id, workspace_id, contact_id, full_name, email, phone, location, linkedin_url,
			portfolio_url, cover_note, resume_attachment_id, cover_letter_attachment, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, workspace_id, contact_id, full_name, email, phone, location, linkedin_url,
			portfolio_url, cover_note, resume_attachment_id, cover_letter_attachment, created_at, updated_at
	`),
		applicant.ID,
		applicant.WorkspaceID,
		applicant.ContactID,
		applicant.FullName,
		applicant.Email,
		applicant.Phone,
		applicant.Location,
		applicant.LinkedInURL,
		applicant.PortfolioURL,
		applicant.CoverNote,
		applicant.ResumeAttachmentID,
		applicant.CoverLetterAttachment,
		applicant.CreatedAt,
		applicant.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("create applicant: %w", err)
	}
	return saved, nil
}

func (s *Store) CreateApplication(ctx context.Context, application *Application) (*Application, error) {
	if application == nil {
		return nil, fmt.Errorf("application is required")
	}
	if application.ID == "" {
		application.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if application.AppliedAt.IsZero() {
		application.AppliedAt = now
	}
	if application.LastStageChangedAt.IsZero() {
		application.LastStageChangedAt = application.AppliedAt
	}
	if application.CreatedAt.IsZero() {
		application.CreatedAt = now
	}
	application.UpdatedAt = now

	saved := &Application{}
	if err := s.db.Get(ctx).GetContext(ctx, saved, s.query(`
		INSERT INTO ${SCHEMA_NAME}.applications (
			id, workspace_id, vacancy_id, applicant_id, case_id, contact_id, form_submission_id,
			source, stage, applied_at, last_stage_changed_at, reviewed_at, hired_at, rejected_at,
			withdrawn_at, rejection_reason, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, workspace_id, vacancy_id, applicant_id, case_id, contact_id, form_submission_id,
			source, stage, applied_at, last_stage_changed_at, reviewed_at, hired_at, rejected_at,
			withdrawn_at, rejection_reason, created_at, updated_at
	`),
		application.ID,
		application.WorkspaceID,
		application.VacancyID,
		application.ApplicantID,
		application.CaseID,
		application.ContactID,
		application.FormSubmissionID,
		application.Source,
		string(application.Stage),
		application.AppliedAt,
		application.LastStageChangedAt,
		application.ReviewedAt,
		application.HiredAt,
		application.RejectedAt,
		application.WithdrawnAt,
		application.RejectionReason,
		application.CreatedAt,
		application.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("create application: %w", err)
	}
	return saved, nil
}

func (s *Store) GetApplication(ctx context.Context, workspaceID, applicationID string) (*Application, error) {
	application := &Application{}
	if err := s.db.Get(ctx).GetContext(ctx, application, s.query(`
		SELECT id, workspace_id, vacancy_id, applicant_id, case_id, contact_id, form_submission_id,
			source, stage, applied_at, last_stage_changed_at, reviewed_at, hired_at, rejected_at,
			withdrawn_at, rejection_reason, created_at, updated_at
		FROM ${SCHEMA_NAME}.applications
		WHERE workspace_id = ? AND id = ?
	`), workspaceID, applicationID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("application not found")
		}
		return nil, fmt.Errorf("get application: %w", err)
	}
	return application, nil
}

func (s *Store) SaveApplication(ctx context.Context, application *Application) (*Application, error) {
	if application == nil {
		return nil, fmt.Errorf("application is required")
	}
	application.UpdatedAt = time.Now().UTC()
	saved := &Application{}
	if err := s.db.Get(ctx).GetContext(ctx, saved, s.query(`
		UPDATE ${SCHEMA_NAME}.applications
		SET case_id = ?, contact_id = ?, form_submission_id = ?, source = ?, stage = ?,
			applied_at = ?, last_stage_changed_at = ?, reviewed_at = ?, hired_at = ?, rejected_at = ?,
			withdrawn_at = ?, rejection_reason = ?, updated_at = ?
		WHERE workspace_id = ? AND id = ?
		RETURNING id, workspace_id, vacancy_id, applicant_id, case_id, contact_id, form_submission_id,
			source, stage, applied_at, last_stage_changed_at, reviewed_at, hired_at, rejected_at,
			withdrawn_at, rejection_reason, created_at, updated_at
	`),
		application.CaseID,
		application.ContactID,
		application.FormSubmissionID,
		application.Source,
		string(application.Stage),
		application.AppliedAt,
		application.LastStageChangedAt,
		application.ReviewedAt,
		application.HiredAt,
		application.RejectedAt,
		application.WithdrawnAt,
		application.RejectionReason,
		application.UpdatedAt,
		application.WorkspaceID,
		application.ID,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("application not found")
		}
		return nil, fmt.Errorf("save application: %w", err)
	}
	return saved, nil
}

func (s *Store) GetApplicant(ctx context.Context, workspaceID, applicantID string) (*Applicant, error) {
	applicant := &Applicant{}
	if err := s.db.Get(ctx).GetContext(ctx, applicant, s.query(`
		SELECT id, workspace_id, contact_id, full_name, email, phone, location, linkedin_url,
			portfolio_url, cover_note, resume_attachment_id, cover_letter_attachment, created_at, updated_at
		FROM ${SCHEMA_NAME}.applicants
		WHERE workspace_id = ? AND id = ?
	`), workspaceID, applicantID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("applicant not found")
		}
		return nil, fmt.Errorf("get applicant: %w", err)
	}
	return applicant, nil
}

func (s *Store) ListApplications(ctx context.Context, workspaceID, vacancyID string) ([]Application, error) {
	args := []any{workspaceID}
	query := `
		SELECT id, workspace_id, vacancy_id, applicant_id, case_id, contact_id, form_submission_id,
			source, stage, applied_at, last_stage_changed_at, reviewed_at, hired_at, rejected_at,
			withdrawn_at, rejection_reason, created_at, updated_at
		FROM ${SCHEMA_NAME}.applications
		WHERE workspace_id = ?
	`
	if strings.TrimSpace(vacancyID) != "" {
		query += ` AND vacancy_id = ?`
		args = append(args, vacancyID)
	}
	query += ` ORDER BY last_stage_changed_at DESC, created_at DESC`

	var applications []Application
	if err := s.db.Get(ctx).SelectContext(ctx, &applications, s.query(query), args...); err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	return applications, nil
}

func (s *Store) ListCandidateProfiles(ctx context.Context, workspaceID, vacancyID string) ([]CandidateProfile, error) {
	applications, err := s.ListApplications(ctx, workspaceID, vacancyID)
	if err != nil {
		return nil, err
	}

	profiles := make([]CandidateProfile, 0, len(applications))
	for _, application := range applications {
		applicant, err := s.GetApplicant(ctx, workspaceID, application.ApplicantID)
		if err != nil {
			return nil, err
		}
		notes, err := s.ListRecruiterNotes(ctx, workspaceID, application.ID)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, CandidateProfile{
			Applicant:   *applicant,
			Application: application,
			Notes:       notes,
		})
	}
	return profiles, nil
}

func (s *Store) AddRecruiterNote(ctx context.Context, workspaceID, applicationID, authorName, authorType, body string) (*RecruiterNote, error) {
	note := &RecruiterNote{
		ID:            uuid.NewString(),
		WorkspaceID:   strings.TrimSpace(workspaceID),
		ApplicationID: strings.TrimSpace(applicationID),
		AuthorName:    strings.TrimSpace(authorName),
		AuthorType:    strings.TrimSpace(authorType),
		Body:          strings.TrimSpace(body),
		CreatedAt:     time.Now().UTC(),
	}
	if note.WorkspaceID == "" || note.ApplicationID == "" || note.Body == "" {
		return nil, fmt.Errorf("workspace, application, and note body are required")
	}
	if note.AuthorType == "" {
		note.AuthorType = "recruiter"
	}

	saved := &RecruiterNote{}
	if err := s.db.Get(ctx).GetContext(ctx, saved, s.query(`
		INSERT INTO ${SCHEMA_NAME}.recruiter_notes (
			id, workspace_id, application_id, author_name, author_type, body, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id, workspace_id, application_id, author_name, author_type, body, created_at
	`),
		note.ID,
		note.WorkspaceID,
		note.ApplicationID,
		note.AuthorName,
		note.AuthorType,
		note.Body,
		note.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("create recruiter note: %w", err)
	}
	return saved, nil
}

func (s *Store) ListRecruiterNotes(ctx context.Context, workspaceID, applicationID string) ([]RecruiterNote, error) {
	var notes []RecruiterNote
	if err := s.db.Get(ctx).SelectContext(ctx, &notes, s.query(`
		SELECT id, workspace_id, application_id, author_name, author_type, body, created_at
		FROM ${SCHEMA_NAME}.recruiter_notes
		WHERE workspace_id = ? AND application_id = ?
		ORDER BY created_at ASC
	`), workspaceID, applicationID); err != nil {
		return nil, fmt.Errorf("list recruiter notes: %w", err)
	}
	return notes, nil
}

func (s *Store) ListStagePresets(ctx context.Context, workspaceID string) ([]StagePreset, error) {
	var presets []StagePreset
	if err := s.db.Get(ctx).SelectContext(ctx, &presets, s.query(`
		SELECT id, workspace_id, slug, name, stages, is_default, created_at, updated_at
		FROM ${SCHEMA_NAME}.stage_presets
		WHERE workspace_id = ?
		ORDER BY is_default DESC, slug ASC
	`), workspaceID); err != nil {
		return nil, fmt.Errorf("list stage presets: %w", err)
	}
	return presets, nil
}

func (s *Store) ListSavedFilters(ctx context.Context, workspaceID string) ([]SavedFilter, error) {
	var filters []SavedFilter
	if err := s.db.Get(ctx).SelectContext(ctx, &filters, s.query(`
		SELECT id, workspace_id, slug, name, criteria, created_at, updated_at
		FROM ${SCHEMA_NAME}.saved_filters
		WHERE workspace_id = ?
		ORDER BY slug ASC
	`), workspaceID); err != nil {
		return nil, fmt.Errorf("list saved filters: %w", err)
	}
	return filters, nil
}

func (s *Store) ensureStagePresets(ctx context.Context, workspaceID string) error {
	var count int
	if err := s.db.Get(ctx).GetContext(ctx, &count, s.query(`
		SELECT COUNT(*) FROM ${SCHEMA_NAME}.stage_presets WHERE workspace_id = ?
	`), workspaceID); err != nil {
		return fmt.Errorf("count stage presets: %w", err)
	}
	if count > 0 {
		return nil
	}

	now := time.Now().UTC()
	for _, preset := range defaultStagePresets(workspaceID, now) {
		if _, err := s.db.Get(ctx).ExecContext(ctx, s.query(`
			INSERT INTO ${SCHEMA_NAME}.stage_presets (
				id, workspace_id, slug, name, stages, is_default, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`),
			preset.ID,
			preset.WorkspaceID,
			preset.Slug,
			preset.Name,
			pq.Array([]string(preset.Stages)),
			preset.IsDefault,
			preset.CreatedAt,
			preset.UpdatedAt,
		); err != nil {
			return fmt.Errorf("seed stage preset %s: %w", preset.Slug, err)
		}
	}
	return nil
}

func (s *Store) ensureSavedFilters(ctx context.Context, workspaceID string) error {
	var count int
	if err := s.db.Get(ctx).GetContext(ctx, &count, s.query(`
		SELECT COUNT(*) FROM ${SCHEMA_NAME}.saved_filters WHERE workspace_id = ?
	`), workspaceID); err != nil {
		return fmt.Errorf("count saved filters: %w", err)
	}
	if count > 0 {
		return nil
	}

	now := time.Now().UTC()
	for _, filter := range defaultSavedFilters(workspaceID, now) {
		if _, err := s.db.Get(ctx).ExecContext(ctx, s.query(`
			INSERT INTO ${SCHEMA_NAME}.saved_filters (
				id, workspace_id, slug, name, criteria, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`),
			filter.ID,
			filter.WorkspaceID,
			filter.Slug,
			filter.Name,
			filter.Criteria,
			filter.CreatedAt,
			filter.UpdatedAt,
		); err != nil {
			return fmt.Errorf("seed saved filter %s: %w", filter.Slug, err)
		}
	}
	return nil
}

func defaultStagePresets(workspaceID string, now time.Time) []StagePreset {
	return []StagePreset{
		{
			ID:          uuid.NewString(),
			WorkspaceID: workspaceID,
			Slug:        "active-funnel",
			Name:        "Active Funnel",
			Stages:      pq.StringArray{string(atsdomain.ApplicationStageReceived), string(atsdomain.ApplicationStageScreening), string(atsdomain.ApplicationStageInterview), string(atsdomain.ApplicationStageOffer)},
			IsDefault:   true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          uuid.NewString(),
			WorkspaceID: workspaceID,
			Slug:        "decision-ready",
			Name:        "Decision Ready",
			Stages:      pq.StringArray{string(atsdomain.ApplicationStageInterview), string(atsdomain.ApplicationStageOffer)},
			IsDefault:   false,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          uuid.NewString(),
			WorkspaceID: workspaceID,
			Slug:        "closed-outcomes",
			Name:        "Closed Outcomes",
			Stages:      pq.StringArray{string(atsdomain.ApplicationStageHired), string(atsdomain.ApplicationStageRejected), string(atsdomain.ApplicationStageWithdrawn)},
			IsDefault:   false,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func defaultSavedFilters(workspaceID string, now time.Time) []SavedFilter {
	mustJSON := func(value any) json.RawMessage {
		encoded, _ := json.Marshal(value)
		return encoded
	}
	return []SavedFilter{
		{
			ID:          uuid.NewString(),
			WorkspaceID: workspaceID,
			Slug:        "needs-review",
			Name:        "Needs Review",
			Criteria: mustJSON(map[string]any{
				"stages": []string{string(atsdomain.ApplicationStageReceived), string(atsdomain.ApplicationStageScreening)},
			}),
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:          uuid.NewString(),
			WorkspaceID: workspaceID,
			Slug:        "open-jobs",
			Name:        "Open Jobs",
			Criteria: mustJSON(map[string]any{
				"statuses": []string{string(atsdomain.VacancyStatusOpen), string(atsdomain.VacancyStatusPaused)},
			}),
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

func vacancyFromDomain(v *atsdomain.Vacancy) *Vacancy {
	if v == nil {
		return nil
	}
	return &Vacancy{
		ID:                  v.ID,
		WorkspaceID:         v.WorkspaceID,
		Slug:                v.Slug,
		Title:               v.Title,
		Team:                v.Team,
		Location:            v.Location,
		WorkMode:            v.WorkMode,
		EmploymentType:      v.EmploymentType,
		Status:              v.Status,
		Summary:             v.Summary,
		Description:         v.Description,
		ApplicationFormSlug: v.ApplicationFormSlug,
		CaseQueueSlug:       v.CaseQueueSlug,
		CareersPath:         v.CareersPath,
		PublishedAt:         v.PublishedAt,
		ClosedAt:            v.ClosedAt,
		CreatedAt:           v.CreatedAt,
		UpdatedAt:           v.UpdatedAt,
	}
}

func (v Vacancy) toDomain() *atsdomain.Vacancy {
	return &atsdomain.Vacancy{
		ID:                  v.ID,
		WorkspaceID:         v.WorkspaceID,
		Slug:                v.Slug,
		Title:               v.Title,
		Team:                v.Team,
		Location:            v.Location,
		WorkMode:            v.WorkMode,
		EmploymentType:      v.EmploymentType,
		Status:              v.Status,
		Summary:             v.Summary,
		Description:         v.Description,
		ApplicationFormSlug: v.ApplicationFormSlug,
		CaseQueueSlug:       v.CaseQueueSlug,
		CareersPath:         v.CareersPath,
		PublishedAt:         v.PublishedAt,
		ClosedAt:            v.ClosedAt,
		CreatedAt:           v.CreatedAt,
		UpdatedAt:           v.UpdatedAt,
	}
}

func applicantFromDomain(a *atsdomain.Applicant) *Applicant {
	if a == nil {
		return nil
	}
	return &Applicant{
		ID:                    a.ID,
		WorkspaceID:           a.WorkspaceID,
		FullName:              a.FullName,
		Email:                 a.Email,
		Phone:                 a.Phone,
		Location:              a.Location,
		LinkedInURL:           a.LinkedInURL,
		PortfolioURL:          a.PortfolioURL,
		CoverNote:             a.CoverNote,
		ResumeAttachmentID:    a.ResumeAttachmentID,
		CoverLetterAttachment: a.CoverLetterAttachment,
		CreatedAt:             a.CreatedAt,
		UpdatedAt:             a.UpdatedAt,
	}
}

func (a Application) toDomain() *atsdomain.Application {
	return &atsdomain.Application{
		ID:                 a.ID,
		WorkspaceID:        a.WorkspaceID,
		VacancyID:          a.VacancyID,
		ApplicantID:        a.ApplicantID,
		CaseID:             a.CaseID,
		ContactID:          a.ContactID,
		FormSubmissionID:   a.FormSubmissionID,
		Source:             a.Source,
		Stage:              a.Stage,
		AppliedAt:          a.AppliedAt,
		LastStageChangedAt: a.LastStageChangedAt,
		ReviewedAt:         a.ReviewedAt,
		HiredAt:            a.HiredAt,
		RejectedAt:         a.RejectedAt,
		WithdrawnAt:        a.WithdrawnAt,
		RejectionReason:    a.RejectionReason,
	}
}

func applicationFromDomain(a *atsdomain.Application) *Application {
	if a == nil {
		return nil
	}
	now := time.Now().UTC()
	return &Application{
		ID:                 a.ID,
		WorkspaceID:        a.WorkspaceID,
		VacancyID:          a.VacancyID,
		ApplicantID:        a.ApplicantID,
		CaseID:             a.CaseID,
		ContactID:          a.ContactID,
		FormSubmissionID:   a.FormSubmissionID,
		Source:             a.Source,
		Stage:              a.Stage,
		AppliedAt:          a.AppliedAt,
		LastStageChangedAt: a.LastStageChangedAt,
		ReviewedAt:         a.ReviewedAt,
		HiredAt:            a.HiredAt,
		RejectedAt:         a.RejectedAt,
		WithdrawnAt:        a.WithdrawnAt,
		RejectionReason:    a.RejectionReason,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func (s *Store) query(query string) string {
	return strings.ReplaceAll(query, "${SCHEMA_NAME}", SchemaName)
}
