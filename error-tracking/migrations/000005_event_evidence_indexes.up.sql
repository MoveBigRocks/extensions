-- Agent evidence reads page an issue's events newest first with a stable
-- (timestamp, id) keyset; this index serves both the order and the cursor.
CREATE INDEX IF NOT EXISTS idx_error_events_issue_timeline
    ON ${SCHEMA_NAME}.error_events (issue_id, timestamp DESC, id DESC)
    WHERE issue_id IS NOT NULL;

-- Issue lists filtered by environment and/or release probe each candidate
-- issue's events with an index-only lookup instead of reading event rows.
CREATE INDEX IF NOT EXISTS idx_error_events_issue_environment_release
    ON ${SCHEMA_NAME}.error_events (issue_id, environment, release)
    WHERE issue_id IS NOT NULL;

-- Exact tag lookups (for example request_id = <uuid>) use JSONB containment.
-- jsonb_path_ops indexes key/value pairs only, which is all @> needs. Readers
-- still bound every lookup by workspace, a time window and a page size.
CREATE INDEX IF NOT EXISTS idx_error_events_tags
    ON ${SCHEMA_NAME}.error_events USING GIN (tags jsonb_path_ops);
