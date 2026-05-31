-- Video categories
CREATE TABLE IF NOT EXISTS video_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Videos (long-form: vlogs, cinema, education, etc.)
CREATE TABLE IF NOT EXISTS videos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    video_url TEXT NOT NULL,
    thumbnail_url TEXT DEFAULT '',
    duration_seconds INT DEFAULT 0,
    category_id UUID REFERENCES video_categories(id),
    resolution VARCHAR(10) DEFAULT '',
    views_count INT DEFAULT 0,
    likes_count INT DEFAULT 0,
    comments_count INT DEFAULT 0,
    is_live BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Audio tracks (used by reels)
CREATE TABLE IF NOT EXISTS audio_tracks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    artist TEXT NOT NULL DEFAULT '',
    cover_url TEXT NOT NULL DEFAULT '',
    audio_url TEXT NOT NULL DEFAULT '',
    duration_seconds INT NOT NULL DEFAULT 0,
    uses_count INT NOT NULL DEFAULT 0,
    genre VARCHAR(50) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Reels (short-form TikTok-style)
CREATE TABLE IF NOT EXISTS reels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    caption TEXT DEFAULT '',
    media_urls TEXT[] NOT NULL DEFAULT '{}',
    media_type VARCHAR(20) DEFAULT 'photo',
    audio_track_id UUID REFERENCES audio_tracks(id),
    duration_seconds INT DEFAULT 15,
    views_count INT DEFAULT 0,
    likes_count INT DEFAULT 0,
    comments_count INT DEFAULT 0,
    shares_count INT DEFAULT 0,
    hashtags TEXT[] DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Video views
CREATE TABLE IF NOT EXISTS video_views (
    video_id UUID NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    watched_seconds INT DEFAULT 0,
    viewed_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (video_id, user_id)
);

-- Reel views
CREATE TABLE IF NOT EXISTS reel_views (
    reel_id UUID NOT NULL REFERENCES reels(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    viewed_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (reel_id, user_id)
);

-- Reel likes
CREATE TABLE IF NOT EXISTS reel_likes (
    reel_id UUID NOT NULL REFERENCES reels(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (reel_id, user_id)
);

-- Video likes
CREATE TABLE IF NOT EXISTS video_likes (
    video_id UUID NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (video_id, user_id)
);

CREATE INDEX idx_videos_user_id ON videos(user_id);
CREATE INDEX idx_videos_category_id ON videos(category_id);
CREATE INDEX idx_videos_created_at ON videos(created_at DESC);
CREATE INDEX idx_reels_user_id ON reels(user_id);
CREATE INDEX idx_reels_created_at ON reels(created_at DESC);
CREATE INDEX idx_reels_likes_count ON reels(likes_count DESC);
