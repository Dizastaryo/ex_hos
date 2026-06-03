ALTER TABLE room_messages
    DROP COLUMN IF EXISTS kind,
    DROP COLUMN IF EXISTS attached_media_url;
