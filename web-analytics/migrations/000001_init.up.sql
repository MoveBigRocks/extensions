CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.salts (
    id BIGSERIAL PRIMARY KEY,
    salt BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_salts_created_at
    ON ${SCHEMA_NAME}.salts(created_at DESC);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.properties (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    status TEXT NOT NULL DEFAULT 'active',
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT properties_status_check CHECK (status IN ('active', 'paused'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_properties_domain_unique
    ON ${SCHEMA_NAME}.properties (LOWER(domain));

CREATE INDEX IF NOT EXISTS idx_properties_workspace
    ON ${SCHEMA_NAME}.properties(workspace_id);

CREATE INDEX IF NOT EXISTS idx_properties_install
    ON ${SCHEMA_NAME}.properties(extension_install_id);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.hostname_rules (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.properties(id) ON DELETE CASCADE,
    pattern TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_hostname_rules_property_pattern_unique
    ON ${SCHEMA_NAME}.hostname_rules(property_id, pattern);

CREATE INDEX IF NOT EXISTS idx_hostname_rules_workspace
    ON ${SCHEMA_NAME}.hostname_rules(workspace_id);

CREATE INDEX IF NOT EXISTS idx_hostname_rules_install
    ON ${SCHEMA_NAME}.hostname_rules(extension_install_id);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.goals (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.properties(id) ON DELETE CASCADE,
    goal_type TEXT NOT NULL,
    event_name TEXT,
    page_path TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT goals_type_check CHECK (goal_type IN ('event', 'page')),
    CONSTRAINT goals_shape_check CHECK (
        (goal_type = 'event' AND event_name IS NOT NULL AND page_path IS NULL) OR
        (goal_type = 'page' AND page_path IS NOT NULL AND event_name IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_goals_property
    ON ${SCHEMA_NAME}.goals(property_id);

CREATE INDEX IF NOT EXISTS idx_goals_workspace
    ON ${SCHEMA_NAME}.goals(workspace_id);

CREATE INDEX IF NOT EXISTS idx_goals_install
    ON ${SCHEMA_NAME}.goals(extension_install_id);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.events (
    id BIGSERIAL PRIMARY KEY,
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.properties(id) ON DELETE CASCADE,
    visitor_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    pathname TEXT NOT NULL,
    referrer_source TEXT NOT NULL DEFAULT '',
    utm_source TEXT NOT NULL DEFAULT '',
    utm_medium TEXT NOT NULL DEFAULT '',
    utm_campaign TEXT NOT NULL DEFAULT '',
    country_code TEXT NOT NULL DEFAULT '',
    region TEXT NOT NULL DEFAULT '',
    city TEXT NOT NULL DEFAULT '',
    browser TEXT NOT NULL DEFAULT '',
    os TEXT NOT NULL DEFAULT '',
    device_type TEXT NOT NULL DEFAULT '',
    timestamp TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_events_property_ts
    ON ${SCHEMA_NAME}.events(property_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_events_workspace_ts
    ON ${SCHEMA_NAME}.events(workspace_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_events_install_ts
    ON ${SCHEMA_NAME}.events(extension_install_id, timestamp DESC);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.sessions (
    session_id BIGINT NOT NULL,
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.properties(id) ON DELETE CASCADE,
    visitor_id BIGINT NOT NULL,
    entry_page TEXT NOT NULL DEFAULT '',
    exit_page TEXT NOT NULL DEFAULT '',
    referrer_source TEXT NOT NULL DEFAULT '',
    utm_source TEXT NOT NULL DEFAULT '',
    utm_medium TEXT NOT NULL DEFAULT '',
    utm_campaign TEXT NOT NULL DEFAULT '',
    country_code TEXT NOT NULL DEFAULT '',
    region TEXT NOT NULL DEFAULT '',
    city TEXT NOT NULL DEFAULT '',
    browser TEXT NOT NULL DEFAULT '',
    os TEXT NOT NULL DEFAULT '',
    device_type TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL,
    last_activity TIMESTAMPTZ NOT NULL,
    duration INTEGER NOT NULL DEFAULT 0,
    pageviews INTEGER NOT NULL DEFAULT 0,
    is_bounce BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (property_id, session_id)
);

CREATE INDEX IF NOT EXISTS idx_sessions_property_visitor_activity
    ON ${SCHEMA_NAME}.sessions(property_id, visitor_id, last_activity DESC);

CREATE INDEX IF NOT EXISTS idx_sessions_workspace_activity
    ON ${SCHEMA_NAME}.sessions(workspace_id, last_activity DESC);

CREATE INDEX IF NOT EXISTS idx_sessions_install_activity
    ON ${SCHEMA_NAME}.sessions(extension_install_id, last_activity DESC);
