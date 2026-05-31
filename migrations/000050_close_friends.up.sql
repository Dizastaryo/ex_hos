-- PROFILE-3: «Close friends» feature (Insta). user_id выбирает подмножество
-- followers (или любых юзеров) как close friends — публикует «for CF» story,
-- видят только они. Зелёный bordered ring в stories-row.
CREATE TABLE close_friends (
    owner_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    friend_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (owner_id, friend_id),
    CHECK (owner_id <> friend_id)
);

CREATE INDEX idx_close_friends_friend ON close_friends(friend_id);

-- Stories с этим флагом видны только close_friends. Если false — нормальные
-- followers/public-stories rules.
ALTER TABLE stories
    ADD COLUMN IF NOT EXISTS is_close_friends_only BOOLEAN NOT NULL DEFAULT false;
