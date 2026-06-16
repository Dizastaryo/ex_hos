-- room_message_reactions: хранит реакции (эмодзи) пользователей на сообщения комнаты.
-- Один пользователь — одна реакция на сообщение (составной PK).
CREATE TABLE IF NOT EXISTS room_message_reactions (
    room_message_id UUID        NOT NULL REFERENCES room_messages(id) ON DELETE CASCADE,
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji           TEXT        NOT NULL CHECK (char_length(emoji) BETWEEN 1 AND 32),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_message_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_rmr_message ON room_message_reactions (room_message_id);
