CREATE TABLE ${SCHEMA_NAME}.sources (
 id uuid PRIMARY KEY,
 workspace_id uuid NOT NULL,
 environment text NOT NULL,
 configuration_digest text NOT NULL,
 last_delivery_at timestamptz,
 last_canary_at timestamptz,
 last_snapshot_at timestamptz,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE ${SCHEMA_NAME}.configuration_events (
 id uuid PRIMARY KEY DEFAULT uuidv7(), workspace_id uuid NOT NULL, source_id uuid NOT NULL,
 digest text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE ${SCHEMA_NAME}.deliveries (
 id uuid PRIMARY KEY DEFAULT uuidv7(), workspace_id uuid NOT NULL, source_id uuid NOT NULL,
 digest text NOT NULL, truncated_alerts integer NOT NULL DEFAULT 0,
 received_at timestamptz NOT NULL DEFAULT now(), UNIQUE(workspace_id,source_id,digest)
);
CREATE TABLE ${SCHEMA_NAME}.incidents (
 id uuid PRIMARY KEY DEFAULT uuidv7(), workspace_id uuid NOT NULL, source_id uuid NOT NULL,
 environment text NOT NULL, component text NOT NULL, severity text NOT NULL,
 case_id uuid, case_status text NOT NULL DEFAULT '', signal_state text NOT NULL DEFAULT 'firing',
 owner_id text NOT NULL DEFAULT '', runbook_id text NOT NULL DEFAULT '', canary boolean NOT NULL DEFAULT false,
 revision bigint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(), closed_at timestamptz, last_case_check_at timestamptz
);
CREATE UNIQUE INDEX one_open_incident_per_component ON ${SCHEMA_NAME}.incidents(workspace_id,source_id,component) WHERE closed_at IS NULL;
CREATE INDEX incident_pages ON ${SCHEMA_NAME}.incidents(workspace_id,created_at DESC,id DESC);
CREATE TABLE ${SCHEMA_NAME}.signals (
 id uuid PRIMARY KEY DEFAULT uuidv7(), workspace_id uuid NOT NULL, source_id uuid NOT NULL,
 fingerprint text NOT NULL, starts_at timestamptz NOT NULL, ends_at timestamptz,
 incident_id uuid REFERENCES ${SCHEMA_NAME}.incidents(id), component text NOT NULL,
 alert_name text NOT NULL, severity text NOT NULL, status text NOT NULL CHECK(status IN ('firing','resolved','unknown')),
 received_at timestamptz NOT NULL DEFAULT now(), UNIQUE(workspace_id,source_id,fingerprint,starts_at)
);
CREATE INDEX active_component_signals ON ${SCHEMA_NAME}.signals(workspace_id,source_id,component) WHERE status='firing';
CREATE TABLE ${SCHEMA_NAME}.observations (
 id uuid PRIMARY KEY DEFAULT uuidv7(), workspace_id uuid NOT NULL, source_id uuid NOT NULL, component text NOT NULL,
 observed_at timestamptz NOT NULL, received_at timestamptz NOT NULL DEFAULT now(), payload jsonb NOT NULL,
 UNIQUE(workspace_id,source_id,component,observed_at)
);
CREATE INDEX observation_current ON ${SCHEMA_NAME}.observations(workspace_id,source_id,component,observed_at DESC);
CREATE TABLE ${SCHEMA_NAME}.deployments (
 id uuid PRIMARY KEY DEFAULT uuidv7(), workspace_id uuid NOT NULL, source_id uuid NOT NULL,
 component text NOT NULL, run_id text NOT NULL, starts_at timestamptz NOT NULL, expires_at timestamptz NOT NULL,
 state text NOT NULL, payload jsonb NOT NULL, received_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(workspace_id,source_id,component,run_id,state)
);
CREATE TABLE ${SCHEMA_NAME}.projection_outbox (
 id uuid PRIMARY KEY DEFAULT uuidv7(), workspace_id uuid NOT NULL,
 incident_id uuid NOT NULL REFERENCES ${SCHEMA_NAME}.incidents(id),
 operation_key text NOT NULL, kind text NOT NULL CHECK(kind IN ('create','note')), payload jsonb NOT NULL,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','leased','delivered','quarantined')),
 claim_token uuid, lease_until timestamptz, attempts integer NOT NULL DEFAULT 0,
 retry_at timestamptz NOT NULL DEFAULT now(), last_error text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(), delivered_at timestamptz,
 UNIQUE(workspace_id,operation_key)
);
CREATE INDEX projection_due ON ${SCHEMA_NAME}.projection_outbox(workspace_id,retry_at) WHERE state IN ('pending','leased');
ALTER TABLE ${SCHEMA_NAME}.sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE ${SCHEMA_NAME}.sources FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON ${SCHEMA_NAME}.sources
 AS RESTRICTIVE
 USING (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid)
 WITH CHECK (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid);
ALTER TABLE ${SCHEMA_NAME}.configuration_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE ${SCHEMA_NAME}.configuration_events FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON ${SCHEMA_NAME}.configuration_events
 AS RESTRICTIVE
 USING (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid)
 WITH CHECK (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid);
ALTER TABLE ${SCHEMA_NAME}.deliveries ENABLE ROW LEVEL SECURITY;
ALTER TABLE ${SCHEMA_NAME}.deliveries FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON ${SCHEMA_NAME}.deliveries
 AS RESTRICTIVE
 USING (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid)
 WITH CHECK (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid);
ALTER TABLE ${SCHEMA_NAME}.incidents ENABLE ROW LEVEL SECURITY;
ALTER TABLE ${SCHEMA_NAME}.incidents FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON ${SCHEMA_NAME}.incidents
 AS RESTRICTIVE
 USING (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid)
 WITH CHECK (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid);
ALTER TABLE ${SCHEMA_NAME}.signals ENABLE ROW LEVEL SECURITY;
ALTER TABLE ${SCHEMA_NAME}.signals FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON ${SCHEMA_NAME}.signals
 AS RESTRICTIVE
 USING (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid)
 WITH CHECK (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid);
ALTER TABLE ${SCHEMA_NAME}.observations ENABLE ROW LEVEL SECURITY;
ALTER TABLE ${SCHEMA_NAME}.observations FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON ${SCHEMA_NAME}.observations
 AS RESTRICTIVE
 USING (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid)
 WITH CHECK (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid);
ALTER TABLE ${SCHEMA_NAME}.deployments ENABLE ROW LEVEL SECURITY;
ALTER TABLE ${SCHEMA_NAME}.deployments FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON ${SCHEMA_NAME}.deployments
 AS RESTRICTIVE
 USING (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid)
 WITH CHECK (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid);
ALTER TABLE ${SCHEMA_NAME}.projection_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE ${SCHEMA_NAME}.projection_outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON ${SCHEMA_NAME}.projection_outbox
 AS RESTRICTIVE
 USING (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid)
 WITH CHECK (workspace_id = nullif(current_setting('app.current_workspace_id',true),'')::uuid);
