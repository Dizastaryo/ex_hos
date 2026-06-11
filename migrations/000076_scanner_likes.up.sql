-- Лайки в BLE-сканере
-- Когда A ставит лайк B:
--   → B видит реальный аккаунт A в своём списке "Кто лайкнул"
--   → B может сама открыть профиль A и написать
--   → A НЕ может написать B первым — только ждёт

CREATE TABLE scanner_likes (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    liker_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(liker_id, target_user_id)
);

-- target_user читает входящие лайки (сортировка по времени)
CREATE INDEX idx_scanner_likes_target  ON scanner_likes(target_user_id, created_at DESC);
-- liker проверяет кого уже лайкнул
CREATE INDEX idx_scanner_likes_liker   ON scanner_likes(liker_id);
