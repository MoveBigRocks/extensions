CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.stages (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    color TEXT NOT NULL,
    position INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, slug),
    UNIQUE (workspace_id, position)
);

CREATE INDEX IF NOT EXISTS idx_sales_pipeline_stages_workspace
    ON ${SCHEMA_NAME}.stages(workspace_id, position);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.deals (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    stage_id TEXT NOT NULL REFERENCES ${SCHEMA_NAME}.stages(id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    organization_name TEXT NOT NULL DEFAULT '',
    contact_name TEXT NOT NULL DEFAULT '',
    contact_email TEXT NOT NULL DEFAULT '',
    linked_case_id TEXT NOT NULL DEFAULT '',
    value_cents BIGINT NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'USD',
    close_date DATE,
    win_probability INTEGER NOT NULL DEFAULT 10 CHECK (win_probability >= 0 AND win_probability <= 100),
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sales_pipeline_deals_workspace_stage
    ON ${SCHEMA_NAME}.deals(workspace_id, stage_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_sales_pipeline_deals_workspace_close_date
    ON ${SCHEMA_NAME}.deals(workspace_id, close_date);
