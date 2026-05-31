-- Image / voice attachments for chat messages. URL is server-relative
-- (`/uploads/...`) — frontend joins with apiOrigin. media_type is small enum.
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS attached_media_url TEXT NOT NULL DEFAULT '';
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS attached_media_type VARCHAR(10) NOT NULL DEFAULT '';
