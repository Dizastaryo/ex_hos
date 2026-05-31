-- Reply/quote messages. reply_to_message_id ссылается на оригинал.
-- ON DELETE SET NULL — если оригинал удалили, reply сохраняется но
-- становится «orphan» (UI рендерит «сообщение удалено» как fallback).

ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS reply_to_message_id UUID
        REFERENCES messages(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_messages_reply_to
    ON messages(reply_to_message_id)
    WHERE reply_to_message_id IS NOT NULL;
