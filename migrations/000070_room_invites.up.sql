CREATE TABLE room_invites (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id     UUID        NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    inviter_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      VARCHAR(10) NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending','accepted','declined')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (room_id, invitee_id)
);
CREATE INDEX idx_room_invites_invitee ON room_invites(invitee_id, status);
