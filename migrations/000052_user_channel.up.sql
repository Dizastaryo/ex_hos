-- VIDEO-4: channel fields для блогеров. Тап на автора видео → профиль
-- открывается как «канал»: hero-баннер сверху + about-текст + default-таб = Videos.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS channel_about  TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS channel_banner_url TEXT NOT NULL DEFAULT '';
