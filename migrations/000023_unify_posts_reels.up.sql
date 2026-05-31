-- Unify model: every publication is a "post". Audio tracks (used to be a
-- reels-only feature) become an optional column on posts. Existing reels are
-- migrated as posts so seeded video content stays visible in /explore. After
-- migration the reels table is dropped — UI/code path moves to POST /posts.

ALTER TABLE posts
    ADD COLUMN IF NOT EXISTS audio_track_id UUID
        REFERENCES audio_tracks(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_posts_audio_track
    ON posts(audio_track_id) WHERE audio_track_id IS NOT NULL;

-- Migrate reels → posts. Caption + hashtags concatenated (so #tags surface in
-- captions, where the rest of the codebase already parses them). Counters
-- are reset (likes/comments/saves are tracked via separate tables in posts).
INSERT INTO posts (
    id, user_id, caption, media_urls, media_types, location,
    likes_count, comments_count, saves_count,
    created_at, updated_at, audio_track_id
)
SELECT
    r.id,
    r.user_id,
    CASE
        WHEN array_length(r.hashtags, 1) > 0
            THEN trim(both ' ' from coalesce(r.caption,'')) ||
                 E'\n\n' ||
                 (SELECT string_agg('#' || h, ' ') FROM unnest(r.hashtags) AS h)
        ELSE coalesce(r.caption,'')
    END                  AS caption,
    r.media_urls         AS media_urls,
    ARRAY[r.media_type]  AS media_types,  -- single-video reels → 1-element array
    ''                   AS location,
    r.likes_count        AS likes_count,
    r.comments_count     AS comments_count,
    0                    AS saves_count,
    r.created_at         AS created_at,
    r.created_at         AS updated_at,
    r.audio_track_id     AS audio_track_id
FROM reels r
WHERE NOT EXISTS (SELECT 1 FROM posts p WHERE p.id = r.id);

-- Cascade: likes/comments/views/shares on reels are tied to that table; they
-- die with it. New post-based engagement starts fresh on the migrated rows.
DROP TABLE IF EXISTS reels CASCADE;
