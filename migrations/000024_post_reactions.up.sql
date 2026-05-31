-- Persisted emoji reactions on posts (mirrors message_reactions for chat).
-- One reaction per (post, user) — UPSERT replaces existing emoji.
CREATE TABLE IF NOT EXISTS post_reactions (
    post_id    UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji      VARCHAR(16) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (post_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_post_reactions_post
    ON post_reactions(post_id);
