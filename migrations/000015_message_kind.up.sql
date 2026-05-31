ALTER TABLE messages ADD COLUMN IF NOT EXISTS kind VARCHAR(20) NOT NULL DEFAULT 'text';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS attached_post_id UUID REFERENCES posts(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_messages_attached_post ON messages(attached_post_id) WHERE attached_post_id IS NOT NULL;

-- Allow text to be empty when message attaches a post.
ALTER TABLE messages ALTER COLUMN text DROP NOT NULL;
ALTER TABLE messages ALTER COLUMN text SET DEFAULT '';
