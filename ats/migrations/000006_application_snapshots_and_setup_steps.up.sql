ALTER TABLE IF EXISTS ${SCHEMA_NAME}.applications
    ADD COLUMN IF NOT EXISTS submission_full_name TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.applications
    ADD COLUMN IF NOT EXISTS submission_email TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.applications
    ADD COLUMN IF NOT EXISTS submission_phone TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.applications
    ADD COLUMN IF NOT EXISTS submission_location TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.applications
    ADD COLUMN IF NOT EXISTS submission_linkedin_url TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.applications
    ADD COLUMN IF NOT EXISTS submission_portfolio_url TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.applications
    ADD COLUMN IF NOT EXISTS submission_cover_note TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.applications
    ADD COLUMN IF NOT EXISTS submission_resume_attachment_id TEXT NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.applications
    ADD COLUMN IF NOT EXISTS submission_cover_letter_attachment TEXT NOT NULL DEFAULT '';

UPDATE ${SCHEMA_NAME}.applications AS applications
SET submission_full_name = COALESCE(NULLIF(applications.submission_full_name, ''), applicants.full_name, ''),
    submission_email = COALESCE(NULLIF(applications.submission_email, ''), applicants.email, ''),
    submission_phone = COALESCE(NULLIF(applications.submission_phone, ''), applicants.phone, ''),
    submission_location = COALESCE(NULLIF(applications.submission_location, ''), applicants.location, ''),
    submission_linkedin_url = COALESCE(NULLIF(applications.submission_linkedin_url, ''), applicants.linkedin_url, ''),
    submission_portfolio_url = COALESCE(NULLIF(applications.submission_portfolio_url, ''), applicants.portfolio_url, ''),
    submission_cover_note = COALESCE(NULLIF(applications.submission_cover_note, ''), applicants.cover_note, ''),
    submission_resume_attachment_id = COALESCE(NULLIF(applications.submission_resume_attachment_id, ''), applicants.resume_attachment_id, ''),
    submission_cover_letter_attachment = COALESCE(NULLIF(applications.submission_cover_letter_attachment, ''), applicants.cover_letter_attachment, '')
FROM ${SCHEMA_NAME}.applicants AS applicants
WHERE applications.applicant_id = applicants.id
  AND applications.workspace_id = applicants.workspace_id;

CREATE INDEX IF NOT EXISTS idx_ats_applications_workspace_resume
    ON ${SCHEMA_NAME}.applications(workspace_id, submission_resume_attachment_id);

ALTER TABLE IF EXISTS ${SCHEMA_NAME}.careers_setup_states
    ADD COLUMN IF NOT EXISTS confirmed_steps TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[];
