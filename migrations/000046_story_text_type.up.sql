-- STORY-1: text-only stories. media_url становится опциональным (NULL для text),
-- bg_color хранит фоновое оформление: hex (#FF5A3C) или название preset-градиента
-- (sunset / ocean / forest / mono). text_overlay переиспользуется как контент текста.
ALTER TABLE stories
    ADD COLUMN bg_color VARCHAR(40) NOT NULL DEFAULT '';

-- media_url по historical-схеме NOT NULL. Снимаем чтобы text-сторис могли быть без media.
ALTER TABLE stories
    ALTER COLUMN media_url DROP NOT NULL;
