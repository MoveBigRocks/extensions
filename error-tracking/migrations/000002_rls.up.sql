ALTER TABLE ${SCHEMA_NAME}.projects ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS projects_tenant_isolation ON ${SCHEMA_NAME}.projects;
CREATE POLICY projects_tenant_isolation ON ${SCHEMA_NAME}.projects
    USING (workspace_id = public.current_workspace_id())
    WITH CHECK (workspace_id = public.current_workspace_id());

ALTER TABLE ${SCHEMA_NAME}.issues ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS issues_tenant_isolation ON ${SCHEMA_NAME}.issues;
CREATE POLICY issues_tenant_isolation ON ${SCHEMA_NAME}.issues
    USING (workspace_id = public.current_workspace_id())
    WITH CHECK (workspace_id = public.current_workspace_id());

ALTER TABLE ${SCHEMA_NAME}.error_events ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS error_events_tenant_isolation ON ${SCHEMA_NAME}.error_events;
CREATE POLICY error_events_tenant_isolation ON ${SCHEMA_NAME}.error_events
    USING (workspace_id = public.current_workspace_id())
    WITH CHECK (workspace_id = public.current_workspace_id());

ALTER TABLE ${SCHEMA_NAME}.alerts ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS alerts_tenant_isolation ON ${SCHEMA_NAME}.alerts;
CREATE POLICY alerts_tenant_isolation ON ${SCHEMA_NAME}.alerts
    USING (workspace_id = public.current_workspace_id())
    WITH CHECK (workspace_id = public.current_workspace_id());

ALTER TABLE ${SCHEMA_NAME}.project_stats ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS project_stats_tenant_isolation ON ${SCHEMA_NAME}.project_stats;
CREATE POLICY project_stats_tenant_isolation ON ${SCHEMA_NAME}.project_stats
    USING (workspace_id = public.current_workspace_id())
    WITH CHECK (workspace_id = public.current_workspace_id());

ALTER TABLE ${SCHEMA_NAME}.issue_stats ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS issue_stats_tenant_isolation ON ${SCHEMA_NAME}.issue_stats;
CREATE POLICY issue_stats_tenant_isolation ON ${SCHEMA_NAME}.issue_stats
    USING (workspace_id = public.current_workspace_id())
    WITH CHECK (workspace_id = public.current_workspace_id());

ALTER TABLE ${SCHEMA_NAME}.git_repos ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS git_repos_tenant_isolation ON ${SCHEMA_NAME}.git_repos;
CREATE POLICY git_repos_tenant_isolation ON ${SCHEMA_NAME}.git_repos
    USING (workspace_id = public.current_workspace_id())
    WITH CHECK (workspace_id = public.current_workspace_id());
