ALTER TABLE IF EXISTS ${SCHEMA_NAME}.vacancies
    ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'job';

UPDATE ${SCHEMA_NAME}.vacancies
SET kind = 'job'
WHERE COALESCE(NULLIF(kind, ''), '') = '';

CREATE INDEX IF NOT EXISTS idx_ats_vacancies_workspace_kind
    ON ${SCHEMA_NAME}.vacancies(workspace_id, kind, updated_at DESC);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.careers_media_assets (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    purpose TEXT NOT NULL DEFAULT 'other',
    filename TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    artifact_path TEXT NOT NULL DEFAULT '',
    public_url TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ats_careers_media_assets_workspace
    ON ${SCHEMA_NAME}.careers_media_assets(workspace_id, created_at DESC, id ASC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ats_careers_media_assets_workspace_artifact_path
    ON ${SCHEMA_NAME}.careers_media_assets(workspace_id, artifact_path);
