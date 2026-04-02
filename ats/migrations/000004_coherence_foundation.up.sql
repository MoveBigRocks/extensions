ALTER TABLE IF EXISTS ${SCHEMA_NAME}.vacancies
    ADD COLUMN IF NOT EXISTS case_queue_id TEXT NOT NULL DEFAULT '';

UPDATE ${SCHEMA_NAME}.vacancies AS vacancies
SET case_queue_id = queues.id::TEXT
FROM core_service.case_queues AS queues
WHERE vacancies.case_queue_id = ''
  AND vacancies.workspace_id = queues.workspace_id::TEXT
  AND vacancies.case_queue_slug = queues.slug
  AND queues.deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ats_vacancies_workspace_queue
    ON ${SCHEMA_NAME}.vacancies(workspace_id, case_queue_id);

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.applications
    ADD COLUMN IF NOT EXISTS source_kind TEXT NOT NULL DEFAULT 'ats_public';

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.applications
    ADD COLUMN IF NOT EXISTS source_ref_id TEXT NOT NULL DEFAULT '';

UPDATE ${SCHEMA_NAME}.applications
SET source_kind = CASE
        WHEN COALESCE(NULLIF(form_submission_id, ''), '') <> '' THEN 'form_submission'
        ELSE 'ats_public'
    END
WHERE COALESCE(NULLIF(source_kind, ''), '') = ''
   OR (source_kind = 'ats_public' AND COALESCE(NULLIF(form_submission_id, ''), '') <> '');

UPDATE ${SCHEMA_NAME}.applications
SET source_ref_id = COALESCE(NULLIF(form_submission_id, ''), source_ref_id, '')
WHERE COALESCE(NULLIF(source_ref_id, ''), '') = ''
  AND COALESCE(NULLIF(form_submission_id, ''), '') <> '';

CREATE INDEX IF NOT EXISTS idx_ats_applications_workspace_source
    ON ${SCHEMA_NAME}.applications(workspace_id, source_kind, last_stage_changed_at DESC);

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.careers_site_profiles
    ADD COLUMN IF NOT EXISTS privacy_policy_url TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.careers_site_profiles
    ADD COLUMN IF NOT EXISTS custom_css TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.careers_site_profiles
    ADD COLUMN IF NOT EXISTS custom_css_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.careers_site_profiles
    ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.careers_setup_states (
    workspace_id TEXT PRIMARY KEY,
    current_step TEXT NOT NULL DEFAULT 'brand',
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
