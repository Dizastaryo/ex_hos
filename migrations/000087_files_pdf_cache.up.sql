-- Кэш PDF-версии для конвертированных форматов (docx, rtf, odt, fb2, pptx, odp).
-- Заполняется при первом открытии через GET /files/:id/pdf.
-- NULL = конвертация ещё не выполнялась.
ALTER TABLE files ADD COLUMN IF NOT EXISTS pdf_cache_url TEXT;
