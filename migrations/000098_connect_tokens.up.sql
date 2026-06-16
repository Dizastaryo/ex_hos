-- Connect tokens: short-lived QR codes for establishing chats via physical contact.
-- Flow: user shows QR → other user scans → POST /connect/accept → chat created.
CREATE TABLE IF NOT EXISTS connect_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token       TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used        BOOLEAN NOT NULL DEFAULT false,
    used_by     UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_connect_tokens_token ON connect_tokens(token);
CREATE INDEX IF NOT EXISTS idx_connect_tokens_expires ON connect_tokens(expires_at);

-- Scanner proximity heartbeats: client reports visible BLE devices periodically.
-- Used to validate that two users are physically near each other during QR connect.
CREATE TABLE IF NOT EXISTS scanner_heartbeats (
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    visible_hash  TEXT NOT NULL,  -- device_public_id of a visible device
    reported_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scanner_heartbeats_user ON scanner_heartbeats(user_id, reported_at);
CREATE INDEX IF NOT EXISTS idx_scanner_heartbeats_hash ON scanner_heartbeats(visible_hash, reported_at);
