-- Fix for 000112: it declared live_streams.user_id / live_stream_viewers.user_id
-- as BIGINT referencing users(id), but users.id is UUID. The type mismatch made
-- the CREATE TABLE fail, so the tables were never created — yet the migration
-- version still advanced (DB ended up at 114 with no live_streams table).
--
-- Recreate the tables idempotently with the correct UUID type. Safe to run on
-- fresh DBs (000112 already created them → IF NOT EXISTS no-ops) and on existing
-- DBs stuck without the tables (creates them).
CREATE TABLE IF NOT EXISTS live_streams (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title        TEXT        NOT NULL DEFAULT '',
    status       TEXT        NOT NULL DEFAULT 'live',
    viewer_count INT         NOT NULL DEFAULT 0,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at     TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS live_stream_viewers (
    stream_id UUID NOT NULL REFERENCES live_streams(id) ON DELETE CASCADE,
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (stream_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_live_streams_status  ON live_streams(status) WHERE status = 'live';
CREATE INDEX IF NOT EXISTS idx_live_streams_user_id ON live_streams(user_id, status);
