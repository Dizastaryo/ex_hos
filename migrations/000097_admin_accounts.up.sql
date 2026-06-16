-- Admin accounts: login/password auth for admin panel (replaces phone+OTP).
CREATE TABLE IF NOT EXISTS admin_accounts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    login      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    -- links to users.id so JWT user_id works with existing Auth middleware
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Create default admin account: login=admin, password=admin123
-- bcrypt hash of "admin123" (cost 10)
INSERT INTO admin_accounts (login, password_hash, user_id)
SELECT 'admin',
       '$2b$10$/TK6LHeiw9TKUSmBEziMW.eq5yGXGhQ0aF3weGwoETCNCMfZMNJYe',
       id
FROM users
WHERE is_admin = true
LIMIT 1
ON CONFLICT (login) DO NOTHING;
