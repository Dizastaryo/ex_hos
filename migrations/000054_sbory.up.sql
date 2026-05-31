-- Сборы (Gatherings)

CREATE TABLE sbory (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    host_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type         TEXT NOT NULL CHECK (type IN ('offline','online')),
    category     TEXT NOT NULL DEFAULT 'other',
    title        TEXT NOT NULL CHECK (char_length(title) BETWEEN 3 AND 120),
    place        TEXT NOT NULL DEFAULT '',
    description  TEXT NOT NULL DEFAULT '',
    scheduled_at TIMESTAMPTZ,
    flexible_time BOOLEAN NOT NULL DEFAULT FALSE,
    max_slots    INT,                         -- NULL = no limit
    is_live      BOOLEAN NOT NULL DEFAULT FALSE,
    is_cancelled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sbor_members (
    sbor_id    UUID NOT NULL REFERENCES sbory(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'participant' CHECK (role IN ('participant','organizer')),
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (sbor_id, user_id)
);

CREATE INDEX idx_sbory_host     ON sbory(host_id);
CREATE INDEX idx_sbory_created  ON sbory(created_at DESC);
CREATE INDEX idx_sbory_type     ON sbory(type);
CREATE INDEX idx_sbory_category ON sbory(category);
CREATE INDEX idx_sbor_members_user ON sbor_members(user_id);
