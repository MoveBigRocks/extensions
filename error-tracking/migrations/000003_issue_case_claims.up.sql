-- Ledger that makes "create the customer case for this issue" idempotent.
-- The case-events consumer is at-least-once, so the same
-- case.created_for_contact event is redelivered on any partial failure. Core
-- CreateCase takes no idempotency key and the core issue/contact lookup needs a
-- contact id the event does not always carry, so the extension owns the claim:
-- one row per (workspace, issue, contact identity), written before the case is
-- created and completed with the case id afterwards.
CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.issue_case_claims (
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.issues(id) ON DELETE CASCADE,
    dedup_key TEXT NOT NULL,
    case_id UUID,
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, issue_id, dedup_key)
);

CREATE INDEX IF NOT EXISTS idx_issue_case_claims_workspace
    ON ${SCHEMA_NAME}.issue_case_claims(workspace_id);

CREATE INDEX IF NOT EXISTS idx_issue_case_claims_install
    ON ${SCHEMA_NAME}.issue_case_claims(extension_install_id);

CREATE INDEX IF NOT EXISTS idx_issue_case_claims_issue
    ON ${SCHEMA_NAME}.issue_case_claims(issue_id);

ALTER TABLE ${SCHEMA_NAME}.issue_case_claims ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS issue_case_claims_tenant_isolation ON ${SCHEMA_NAME}.issue_case_claims;
CREATE POLICY issue_case_claims_tenant_isolation ON ${SCHEMA_NAME}.issue_case_claims
    USING (workspace_id = public.current_workspace_id())
    WITH CHECK (workspace_id = public.current_workspace_id());
