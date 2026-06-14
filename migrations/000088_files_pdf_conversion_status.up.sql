ALTER TABLE files ADD COLUMN IF NOT EXISTS pdf_conversion_status TEXT NOT NULL DEFAULT 'none';

-- Files already converted: mark as done
UPDATE files SET pdf_conversion_status = 'done' WHERE pdf_cache_url IS NOT NULL;

-- Convertible files without a cache: queue for background conversion
UPDATE files
SET pdf_conversion_status = 'pending'
WHERE pdf_cache_url IS NULL
  AND doc_format IN ('fb2', 'docx', 'rtf', 'odt', 'pptx', 'odp');
