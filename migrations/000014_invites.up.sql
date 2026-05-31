CREATE TABLE IF NOT EXISTS invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inviter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code VARCHAR(12) NOT NULL UNIQUE,
    used_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invites_inviter ON invites(inviter_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_invites_code ON invites(code) WHERE used_by_id IS NULL;
