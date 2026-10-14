-- Accepted raw events are the recovery ledger. Keep recovery scans bounded by
-- the pending set and ordered fairly by the last processing attempt.
CREATE INDEX IF NOT EXISTS idx_error_events_pending_processing
    ON ${SCHEMA_NAME}.error_events ((COALESCE(processed_at, received)), id)
    WHERE issue_id IS NULL;
