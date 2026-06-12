-- Tracks when the user last viewed their received-likes screen.
-- Used to count unseen likes and display badge on bottom nav.
ALTER TABLE users ADD COLUMN IF NOT EXISTS likes_seen_at TIMESTAMPTZ NOT NULL DEFAULT '1970-01-01';
