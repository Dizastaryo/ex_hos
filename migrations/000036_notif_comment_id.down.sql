DROP INDEX IF EXISTS idx_notifications_comment_id;
ALTER TABLE notifications DROP COLUMN IF EXISTS comment_id;
