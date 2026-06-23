CREATE TABLE IF NOT EXISTS user_access (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_a_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_b_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_a_id, user_b_id),
    CHECK (user_a_id < user_b_id)
);

CREATE INDEX IF NOT EXISTS idx_user_access_a ON user_access(user_a_id);
CREATE INDEX IF NOT EXISTS idx_user_access_b ON user_access(user_b_id);
