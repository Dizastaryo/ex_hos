CREATE TABLE IF NOT EXISTS file_ratings (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_id    UUID        NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    rating     SMALLINT    NOT NULL CHECK (rating BETWEEN 1 AND 5),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, file_id)
);

CREATE INDEX IF NOT EXISTS idx_file_ratings_file ON file_ratings (file_id);

-- Add aggregate columns to files table
ALTER TABLE files
    ADD COLUMN IF NOT EXISTS ratings_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS ratings_sum   INT NOT NULL DEFAULT 0;
