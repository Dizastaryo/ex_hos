-- Voice-message metadata: precomputed waveform для рендера bubble'а без
-- декодирования аудио клиентом + duration_seconds от recorder'а (или
-- ffprobe'нутая на upload'е). Оба поля nullable — для обычных text/image
-- сообщений остаются NULL.

ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS media_duration_seconds INTEGER,
    ADD COLUMN IF NOT EXISTS waveform JSONB;
