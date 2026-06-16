CREATE TABLE IF NOT EXISTS file_views (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_id    UUID        NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    viewed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_file_views_user_viewed ON file_views (user_id, viewed_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_file_views_user_file ON file_views (user_id, file_id);
