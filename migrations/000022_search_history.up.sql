-- Per-user search history. UPSERT-friendly: PK on (user_id, query) so re-searching
-- the same term just bumps the timestamp instead of creating duplicates.
CREATE TABLE IF NOT EXISTS search_history (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    query      VARCHAR(120) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, query)
);

CREATE INDEX IF NOT EXISTS idx_search_history_recent
    ON search_history(user_id, created_at DESC);
