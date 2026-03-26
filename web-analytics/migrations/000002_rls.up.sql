ALTER TABLE ${SCHEMA_NAME}.properties ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS properties_tenant_isolation ON ${SCHEMA_NAME}.properties;
CREATE POLICY properties_tenant_isolation ON ${SCHEMA_NAME}.properties
    USING (workspace_id = public.current_workspace_id())
    WITH CHECK (workspace_id = public.current_workspace_id());

ALTER TABLE ${SCHEMA_NAME}.hostname_rules ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS hostname_rules_tenant_isolation ON ${SCHEMA_NAME}.hostname_rules;
CREATE POLICY hostname_rules_tenant_isolation ON ${SCHEMA_NAME}.hostname_rules
    USING (workspace_id = public.current_workspace_id())
    WITH CHECK (workspace_id = public.current_workspace_id());

ALTER TABLE ${SCHEMA_NAME}.goals ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS goals_tenant_isolation ON ${SCHEMA_NAME}.goals;
CREATE POLICY goals_tenant_isolation ON ${SCHEMA_NAME}.goals
    USING (workspace_id = public.current_workspace_id())
    WITH CHECK (workspace_id = public.current_workspace_id());

ALTER TABLE ${SCHEMA_NAME}.events ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS events_tenant_isolation ON ${SCHEMA_NAME}.events;
CREATE POLICY events_tenant_isolation ON ${SCHEMA_NAME}.events
    USING (workspace_id = public.current_workspace_id())
    WITH CHECK (workspace_id = public.current_workspace_id());

ALTER TABLE ${SCHEMA_NAME}.sessions ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS sessions_tenant_isolation ON ${SCHEMA_NAME}.sessions;
CREATE POLICY sessions_tenant_isolation ON ${SCHEMA_NAME}.sessions
    USING (workspace_id = public.current_workspace_id())
    WITH CHECK (workspace_id = public.current_workspace_id());
