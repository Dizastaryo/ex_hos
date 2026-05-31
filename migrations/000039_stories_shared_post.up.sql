-- CHAT-5: share post в сторис.
-- Юзер шейрит существующий пост в свою сторис → frontend POST /stories с
-- media-url'ом первого медиа поста + shared_post_id. На viewer'е сторис
-- рисуется small badge «От @author» tappable → /post/:id.
-- ON DELETE SET NULL — если post удалён, сторис остаётся (media уже
-- скопирован) но ссылка пропадает.

ALTER TABLE stories
    ADD COLUMN IF NOT EXISTS shared_post_id UUID
        REFERENCES posts(id) ON DELETE SET NULL;
