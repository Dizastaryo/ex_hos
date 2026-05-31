-- VIDEO-5: VTT subtitles URL. NULL = без субтитров; frontend chewie/video_player
-- если non-empty, подгружает дорожку и рендерит overlay.
ALTER TABLE videos
    ADD COLUMN IF NOT EXISTS subtitles_url TEXT NOT NULL DEFAULT '';
