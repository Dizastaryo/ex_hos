CREATE TABLE IF NOT EXISTS reading_goals (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    year       SMALLINT    NOT NULL,
    goal_books INT         NOT NULL CHECK (goal_books > 0 AND goal_books <= 1000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, year)
);
