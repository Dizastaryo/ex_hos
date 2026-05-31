DROP INDEX IF EXISTS idx_files_likes_count;
ALTER TABLE files DROP COLUMN IF EXISTS likes_count;
