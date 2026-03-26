CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.projects (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    team_id UUID,
    name TEXT,
    slug TEXT,
    repository TEXT,
    platform TEXT,
    environment TEXT,
    dsn TEXT,
    public_key TEXT,
    secret_key TEXT,
    app_key TEXT,
    project_number BIGINT,
    events_per_hour INTEGER NOT NULL DEFAULT 1000,
    storage_quota_mb INTEGER NOT NULL DEFAULT 1000,
    retention_days INTEGER NOT NULL DEFAULT 90,
    status TEXT NOT NULL DEFAULT 'active',
    event_count BIGINT NOT NULL DEFAULT 0,
    last_event_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT projects_status_check CHECK (status IN ('active', 'paused', 'disabled'))
);

CREATE INDEX IF NOT EXISTS idx_projects_workspace
    ON ${SCHEMA_NAME}.projects(workspace_id);

CREATE INDEX IF NOT EXISTS idx_projects_install
    ON ${SCHEMA_NAME}.projects(extension_install_id);

CREATE INDEX IF NOT EXISTS idx_projects_team
    ON ${SCHEMA_NAME}.projects(team_id);

CREATE INDEX IF NOT EXISTS idx_projects_deleted
    ON ${SCHEMA_NAME}.projects(deleted_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_ws_slug_active_unique
    ON ${SCHEMA_NAME}.projects(workspace_id, slug)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_public_key_unique
    ON ${SCHEMA_NAME}.projects(public_key)
    WHERE public_key IS NOT NULL AND public_key <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_project_number_unique
    ON ${SCHEMA_NAME}.projects(project_number)
    WHERE project_number IS NOT NULL;

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.issues (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.projects(id) ON DELETE CASCADE,
    title TEXT,
    culprit TEXT,
    fingerprint TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unresolved',
    level TEXT,
    type TEXT,
    first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    event_count BIGINT NOT NULL DEFAULT 1,
    user_count BIGINT NOT NULL DEFAULT 0,
    assigned_to UUID,
    resolved_at TIMESTAMPTZ,
    resolved_by UUID,
    resolution TEXT,
    resolution_notes TEXT,
    resolved_in_commit TEXT,
    resolved_in_version TEXT,
    has_related_case BOOLEAN NOT NULL DEFAULT FALSE,
    related_case_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    tags JSONB NOT NULL DEFAULT '{}'::jsonb,
    permalink TEXT,
    short_id TEXT,
    logger TEXT,
    platform TEXT,
    last_event_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_issue_project_fingerprint UNIQUE (project_id, fingerprint)
);

CREATE INDEX IF NOT EXISTS idx_issues_workspace
    ON ${SCHEMA_NAME}.issues(workspace_id);

CREATE INDEX IF NOT EXISTS idx_issues_install
    ON ${SCHEMA_NAME}.issues(extension_install_id);

CREATE INDEX IF NOT EXISTS idx_issues_project
    ON ${SCHEMA_NAME}.issues(project_id);

CREATE INDEX IF NOT EXISTS idx_issues_status
    ON ${SCHEMA_NAME}.issues(status);

CREATE INDEX IF NOT EXISTS idx_issues_short_id
    ON ${SCHEMA_NAME}.issues(short_id);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.error_events (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    event_id TEXT,
    project_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.projects(id) ON DELETE CASCADE,
    issue_id UUID REFERENCES ${SCHEMA_NAME}.issues(id) ON DELETE SET NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    received TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    message TEXT,
    level TEXT,
    logger TEXT,
    platform TEXT,
    environment TEXT,
    release TEXT,
    dist TEXT,
    exception JSONB,
    stacktrace JSONB,
    "user" JSONB,
    request JSONB,
    tags JSONB NOT NULL DEFAULT '{}'::jsonb,
    extra JSONB NOT NULL DEFAULT '{}'::jsonb,
    contexts JSONB NOT NULL DEFAULT '{}'::jsonb,
    breadcrumbs JSONB NOT NULL DEFAULT '[]'::jsonb,
    fingerprint JSONB NOT NULL DEFAULT '[]'::jsonb,
    data_url TEXT,
    size BIGINT,
    processed_at TIMESTAMPTZ,
    grouped_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_error_events_workspace
    ON ${SCHEMA_NAME}.error_events(workspace_id);

CREATE INDEX IF NOT EXISTS idx_error_events_install
    ON ${SCHEMA_NAME}.error_events(extension_install_id);

CREATE INDEX IF NOT EXISTS idx_error_events_project
    ON ${SCHEMA_NAME}.error_events(project_id);

CREATE INDEX IF NOT EXISTS idx_error_events_issue
    ON ${SCHEMA_NAME}.error_events(issue_id);

CREATE INDEX IF NOT EXISTS idx_error_events_timestamp
    ON ${SCHEMA_NAME}.error_events(timestamp DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_error_events_project_event_id_unique
    ON ${SCHEMA_NAME}.error_events(project_id, event_id)
    WHERE event_id IS NOT NULL AND event_id <> '';

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.alerts (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.projects(id) ON DELETE CASCADE,
    name TEXT,
    conditions JSONB NOT NULL DEFAULT '[]'::jsonb,
    frequency BIGINT NOT NULL DEFAULT 0,
    actions JSONB NOT NULL DEFAULT '[]'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    cooldown_minutes INTEGER NOT NULL DEFAULT 60,
    last_triggered TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_workspace
    ON ${SCHEMA_NAME}.alerts(workspace_id);

CREATE INDEX IF NOT EXISTS idx_alerts_install
    ON ${SCHEMA_NAME}.alerts(extension_install_id);

CREATE INDEX IF NOT EXISTS idx_alerts_project
    ON ${SCHEMA_NAME}.alerts(project_id);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.project_stats (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.projects(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    event_count BIGINT NOT NULL DEFAULT 0,
    issue_count BIGINT NOT NULL DEFAULT 0,
    user_count BIGINT NOT NULL DEFAULT 0,
    error_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
    new_issues BIGINT NOT NULL DEFAULT 0,
    resolved_issues BIGINT NOT NULL DEFAULT 0,
    UNIQUE (project_id, date)
);

CREATE INDEX IF NOT EXISTS idx_project_stats_workspace
    ON ${SCHEMA_NAME}.project_stats(workspace_id);

CREATE INDEX IF NOT EXISTS idx_project_stats_install
    ON ${SCHEMA_NAME}.project_stats(extension_install_id);

CREATE INDEX IF NOT EXISTS idx_project_stats_project
    ON ${SCHEMA_NAME}.project_stats(project_id);

CREATE INDEX IF NOT EXISTS idx_project_stats_date
    ON ${SCHEMA_NAME}.project_stats(date DESC);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.issue_stats (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.issues(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    event_count BIGINT NOT NULL DEFAULT 0,
    user_count BIGINT NOT NULL DEFAULT 0,
    first_occurrence TIMESTAMPTZ,
    last_occurrence TIMESTAMPTZ,
    UNIQUE (issue_id, date)
);

CREATE INDEX IF NOT EXISTS idx_issue_stats_workspace
    ON ${SCHEMA_NAME}.issue_stats(workspace_id);

CREATE INDEX IF NOT EXISTS idx_issue_stats_install
    ON ${SCHEMA_NAME}.issue_stats(extension_install_id);

CREATE INDEX IF NOT EXISTS idx_issue_stats_issue
    ON ${SCHEMA_NAME}.issue_stats(issue_id);

CREATE INDEX IF NOT EXISTS idx_issue_stats_date
    ON ${SCHEMA_NAME}.issue_stats(date DESC);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.git_repos (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    application_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.projects(id) ON DELETE CASCADE,
    repo_url TEXT NOT NULL,
    default_branch TEXT NOT NULL DEFAULT 'main',
    access_token TEXT,
    path_prefix TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_git_repos_workspace
    ON ${SCHEMA_NAME}.git_repos(workspace_id);

CREATE INDEX IF NOT EXISTS idx_git_repos_install
    ON ${SCHEMA_NAME}.git_repos(extension_install_id);

CREATE INDEX IF NOT EXISTS idx_git_repos_application
    ON ${SCHEMA_NAME}.git_repos(application_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_git_repos_application_repo_unique
    ON ${SCHEMA_NAME}.git_repos(application_id, repo_url);
