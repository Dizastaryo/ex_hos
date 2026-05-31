-- last_seen_at — apдейтится при connect/disconnect WS-хабом.
-- Используется для "был N мин назад" в UI чата и профиля.
-- NOT NULL DEFAULT NOW() — чтобы новые юзеры не пугали клиента nil'ом.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_users_last_seen_at ON users(last_seen_at DESC);
