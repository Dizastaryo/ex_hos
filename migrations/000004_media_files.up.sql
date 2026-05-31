-- Local file storage with deduplication by content hash
CREATE TABLE IF NOT EXISTS media_files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    hash VARCHAR(64) UNIQUE NOT NULL,
    file_path TEXT NOT NULL,
    mime_type VARCHAR(50) NOT NULL,
    media_type VARCHAR(10) NOT NULL,
    size BIGINT NOT NULL,
    ref_count INT DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_media_files_hash ON media_files(hash);
