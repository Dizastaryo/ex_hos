-- PROFILE-4: restrict feature (мягче чем block). user_id ограничивает
-- restricted_user_id — комменты от restricted_user видны только автору
-- поста + самому commenter'у. Симметрично user_blocks.
CREATE TABLE user_restrictions (
    user_id            UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    restricted_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, restricted_user_id),
    CHECK (user_id <> restricted_user_id)
);

CREATE INDEX idx_user_restrictions_restricted ON user_restrictions(restricted_user_id);
