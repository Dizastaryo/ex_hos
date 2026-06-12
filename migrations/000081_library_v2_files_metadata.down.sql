DROP TRIGGER IF EXISTS files_search_vector_trigger ON files;
DROP FUNCTION IF EXISTS files_search_vector_update();
DROP INDEX IF EXISTS files_search_vector_idx;

ALTER TABLE files
  DROP COLUMN IF EXISTS title,
  DROP COLUMN IF EXISTS author_name,
  DROP COLUMN IF EXISTS language,
  DROP COLUMN IF EXISTS pages_count,
  DROP COLUMN IF EXISTS doc_format,
  DROP COLUMN IF EXISTS extracted_text,
  DROP COLUMN IF EXISTS search_vector;
