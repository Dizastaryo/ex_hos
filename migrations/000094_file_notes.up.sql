CREATE TABLE IF NOT EXISTS file_notes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_id     UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    content     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, file_id)
);

CREATE INDEX IF NOT EXISTS idx_file_notes_user_id ON file_notes(user_id);
CREATE INDEX IF NOT EXISTS idx_file_notes_file_id ON file_notes(file_id);
