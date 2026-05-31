-- Persisted emoji reactions on stories (same shape as post_reactions / message_reactions).
-- One reaction per (story, user) — UPSERT replaces existing emoji.
CREATE TABLE IF NOT EXISTS story_reactions (
    story_id   UUID NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji      VARCHAR(16) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (story_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_story_reactions_story
    ON story_reactions(story_id);
