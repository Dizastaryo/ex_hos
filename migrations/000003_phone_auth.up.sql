-- Remove email/password auth, add phone-based auth with device IDs and profile fields

-- First clear all data since we're changing the schema fundamentally
DELETE FROM notifications;
DELETE FROM likes;
DELETE FROM comments;
DELETE FROM saved_posts;
DELETE FROM story_views;
DELETE FROM highlight_stories;
DELETE FROM highlights;
DELETE FROM stories;
DELETE FROM posts;
DELETE FROM follows;
DELETE FROM refresh_tokens;
DELETE FROM users;

ALTER TABLE users DROP COLUMN IF EXISTS email;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;

ALTER TABLE users ADD COLUMN phone VARCHAR(20) UNIQUE NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN gender VARCHAR(10) DEFAULT '';
ALTER TABLE users ADD COLUMN date_of_birth DATE;
ALTER TABLE users ADD COLUMN device_public_id TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN device_private_id TEXT DEFAULT '';

-- OTP codes table for phone verification
CREATE TABLE IF NOT EXISTS otp_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone VARCHAR(20) NOT NULL,
    code VARCHAR(6) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '5 minutes'),
    used BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_otp_codes_phone ON otp_codes(phone, used, expires_at DESC);
CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);
