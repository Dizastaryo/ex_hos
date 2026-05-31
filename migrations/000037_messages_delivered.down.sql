DROP INDEX IF EXISTS idx_messages_undelivered;
ALTER TABLE messages DROP COLUMN IF EXISTS delivered_at;
