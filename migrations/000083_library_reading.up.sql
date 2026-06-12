-- Sprint 3: reading progress, bookmarks, reading status

CREATE TABLE IF NOT EXISTS reading_progress (
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_id      UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    position     JSONB NOT NULL DEFAULT '{}',
    last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, file_id)
);

CREATE TABLE IF NOT EXISTS file_bookmarks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_id    UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    position   JSONB NOT NULL DEFAULT '{}',
    note       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reading_status (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_id    UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    status     TEXT NOT NULL CHECK (status IN ('want', 'reading', 'done')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, file_id)
);

CREATE INDEX IF NOT EXISTS file_bookmarks_user_file_idx ON file_bookmarks(user_id, file_id);
CREATE INDEX IF NOT EXISTS reading_status_user_idx ON reading_status(user_id);
CREATE INDEX IF NOT EXISTS reading_progress_user_idx ON reading_progress(user_id);
