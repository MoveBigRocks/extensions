CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.feature_requests (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    slug TEXT NOT NULL,
    title TEXT NOT NULL,
    description_markdown TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',
    is_public BOOLEAN NOT NULL DEFAULT TRUE,
    vote_count INTEGER NOT NULL DEFAULT 0,
    comment_count INTEGER NOT NULL DEFAULT 0,
    submitter_name TEXT NOT NULL DEFAULT '',
    submitter_email TEXT NOT NULL DEFAULT '',
    linked_case_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_feature_requests_workspace_public
    ON ${SCHEMA_NAME}.feature_requests(workspace_id, is_public, vote_count DESC, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_feature_requests_workspace_status
    ON ${SCHEMA_NAME}.feature_requests(workspace_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.feature_request_votes (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    feature_request_id TEXT NOT NULL REFERENCES ${SCHEMA_NAME}.feature_requests(id) ON DELETE CASCADE,
    voter_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, feature_request_id, voter_key)
);

CREATE INDEX IF NOT EXISTS idx_feature_request_votes_workspace_request
    ON ${SCHEMA_NAME}.feature_request_votes(workspace_id, feature_request_id, created_at DESC);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.feature_request_comments (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    feature_request_id TEXT NOT NULL REFERENCES ${SCHEMA_NAME}.feature_requests(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    public BOOLEAN NOT NULL DEFAULT TRUE,
    author_name TEXT NOT NULL DEFAULT '',
    author_email TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_feature_request_comments_workspace_request
    ON ${SCHEMA_NAME}.feature_request_comments(workspace_id, feature_request_id, created_at DESC);
