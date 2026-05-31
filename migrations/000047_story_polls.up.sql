-- STORY-3: интерактивный poll-overlay поверх сторис. Хранится как JSONB
-- {question, option_a, option_b}. Голоса — отдельная таблица story_poll_votes.
-- Viewer тапает option → INSERT vote → backend агрегирует %.
ALTER TABLE stories
    ADD COLUMN poll JSONB DEFAULT NULL;

CREATE TABLE story_poll_votes (
    story_id      UUID NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    option_index  SMALLINT NOT NULL CHECK (option_index IN (0, 1)),
    voted_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (story_id, user_id)
);

CREATE INDEX idx_story_poll_votes_story ON story_poll_votes(story_id);
