CREATE TABLE rooms (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id  UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        VARCHAR(10)  NOT NULL DEFAULT 'text' CHECK (type IN ('text','voice')),
    name        VARCHAR(120) NOT NULL,
    description TEXT,
    cover_url   VARCHAR(500),
    is_public   BOOLEAN      NOT NULL DEFAULT true,
    is_active   BOOLEAN      NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE room_participants (
    room_id   UUID        NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_muted  BOOLEAN     NOT NULL DEFAULT false,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (room_id, user_id)
);

CREATE TABLE room_messages (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id    UUID        NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    sender_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_rooms_active     ON rooms(is_active, created_at DESC);
CREATE INDEX idx_room_parts_room  ON room_participants(room_id, joined_at);
CREATE INDEX idx_room_msgs_room   ON room_messages(room_id, created_at DESC);
