CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.public_attachment_uploads (
    token TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    attachment_id TEXT NOT NULL,
    purpose TEXT NOT NULL DEFAULT '',
    consumed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ats_public_attachment_uploads_workspace
    ON ${SCHEMA_NAME}.public_attachment_uploads(workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ats_public_attachment_uploads_attachment
    ON ${SCHEMA_NAME}.public_attachment_uploads(workspace_id, attachment_id);
