-- Whitelist для приватного режима браслета.
-- Владелец браслета сам выбирает кто из взаимных подписчиков
-- может видеть его в private-mode (mode=0x01).
-- Без записи в этой таблице — private_id не резолвится никому.

CREATE TABLE device_private_whitelist (
    owner_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    allowed_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (owner_id, allowed_id),
    -- нельзя добавить самого себя
    CONSTRAINT chk_no_self CHECK (owner_id <> allowed_id)
);

CREATE INDEX idx_priv_whitelist_owner ON device_private_whitelist(owner_id);
