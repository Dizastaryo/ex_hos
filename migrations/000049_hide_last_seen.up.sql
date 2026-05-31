-- PROFILE-6: privacy-toggle для last_seen. Когда true — backend скрывает
-- last_seen_at и is_online от других юзеров (виден только владельцу аккаунта).
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS hide_last_seen BOOLEAN NOT NULL DEFAULT false;
