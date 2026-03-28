CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.vacancies (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    slug TEXT NOT NULL,
    title TEXT NOT NULL,
    team TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    work_mode TEXT NOT NULL DEFAULT 'remote',
    employment_type TEXT NOT NULL DEFAULT 'full_time',
    status TEXT NOT NULL DEFAULT 'draft',
    summary TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    application_form_slug TEXT NOT NULL DEFAULT 'job-application',
    case_queue_slug TEXT NOT NULL DEFAULT '',
    careers_path TEXT NOT NULL DEFAULT '',
    published_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_ats_vacancies_workspace_status
    ON ${SCHEMA_NAME}.vacancies(workspace_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.applicants (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    contact_id TEXT NOT NULL DEFAULT '',
    full_name TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    linkedin_url TEXT NOT NULL DEFAULT '',
    portfolio_url TEXT NOT NULL DEFAULT '',
    cover_note TEXT NOT NULL DEFAULT '',
    resume_attachment_id TEXT NOT NULL DEFAULT '',
    cover_letter_attachment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, email)
);

CREATE INDEX IF NOT EXISTS idx_ats_applicants_workspace_email
    ON ${SCHEMA_NAME}.applicants(workspace_id, email);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.applications (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    vacancy_id TEXT NOT NULL REFERENCES ${SCHEMA_NAME}.vacancies(id) ON DELETE CASCADE,
    applicant_id TEXT NOT NULL REFERENCES ${SCHEMA_NAME}.applicants(id) ON DELETE CASCADE,
    case_id TEXT NOT NULL DEFAULT '',
    contact_id TEXT NOT NULL DEFAULT '',
    form_submission_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'careers_form',
    stage TEXT NOT NULL DEFAULT 'received',
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_stage_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    hired_at TIMESTAMPTZ,
    rejected_at TIMESTAMPTZ,
    withdrawn_at TIMESTAMPTZ,
    rejection_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, vacancy_id, applicant_id)
);

CREATE INDEX IF NOT EXISTS idx_ats_applications_workspace_vacancy
    ON ${SCHEMA_NAME}.applications(workspace_id, vacancy_id, last_stage_changed_at DESC);

CREATE INDEX IF NOT EXISTS idx_ats_applications_workspace_case
    ON ${SCHEMA_NAME}.applications(workspace_id, case_id);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.recruiter_notes (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    application_id TEXT NOT NULL REFERENCES ${SCHEMA_NAME}.applications(id) ON DELETE CASCADE,
    author_name TEXT NOT NULL DEFAULT '',
    author_type TEXT NOT NULL DEFAULT 'recruiter',
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ats_recruiter_notes_workspace_application
    ON ${SCHEMA_NAME}.recruiter_notes(workspace_id, application_id, created_at DESC);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.stage_presets (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    stages TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_ats_stage_presets_workspace
    ON ${SCHEMA_NAME}.stage_presets(workspace_id, is_default DESC, slug);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.saved_filters (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    criteria JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_ats_saved_filters_workspace
    ON ${SCHEMA_NAME}.saved_filters(workspace_id, slug);
