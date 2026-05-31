-- We don't restore the reels table on rollback — data was merged into posts
-- and rolling that out cleanly would require knowing which posts originated
-- from reels. Just drop the new column.
DROP INDEX IF EXISTS idx_posts_audio_track;
ALTER TABLE posts DROP COLUMN IF EXISTS audio_track_id;
