-- Sprint 1: расширенные метаданные файлов + поиск-вектор
ALTER TABLE files
  ADD COLUMN IF NOT EXISTS title          TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS author_name    TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS language       TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS pages_count    INT  NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS doc_format     TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS extracted_text TEXT,
  ADD COLUMN IF NOT EXISTS search_vector  TSVECTOR;

-- Backfill title из filename для существующих строк
UPDATE files SET title = filename WHERE title = '';

-- Backfill doc_format из расширения файла
UPDATE files
SET doc_format = lower(substring(filename FROM '\.([^.]+)$'))
WHERE doc_format = ''
  AND filename ~ '\.[a-zA-Z0-9]+$';

-- Функция обновления поискового вектора
CREATE OR REPLACE FUNCTION files_search_vector_update() RETURNS TRIGGER AS $$
BEGIN
  NEW.search_vector := to_tsvector('simple',
    coalesce(NEW.title, '')       || ' ' ||
    coalesce(NEW.author_name, '') || ' ' ||
    coalesce(NEW.description, '')
  );
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Триггер: обновляем search_vector при вставке/изменении
DROP TRIGGER IF EXISTS files_search_vector_trigger ON files;
CREATE TRIGGER files_search_vector_trigger
  BEFORE INSERT OR UPDATE OF title, author_name, description ON files
  FOR EACH ROW EXECUTE FUNCTION files_search_vector_update();

-- Backfill search_vector для существующих строк
UPDATE files SET search_vector = to_tsvector('simple',
  coalesce(title, '') || ' ' ||
  coalesce(author_name, '') || ' ' ||
  coalesce(description, '')
);

-- GIN индекс для полнотекстового поиска
CREATE INDEX IF NOT EXISTS files_search_vector_idx ON files USING GIN (search_vector);
