ALTER TABLE room_messages
    ADD COLUMN IF NOT EXISTS kind               VARCHAR(30)  NOT NULL DEFAULT 'text',
    ADD COLUMN IF NOT EXISTS attached_media_url TEXT         NOT NULL DEFAULT '';
