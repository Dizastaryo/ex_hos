-- Sprint 1: расширить file_categories + засеять 5 стандартных категорий
ALTER TABLE file_categories
  ADD COLUMN IF NOT EXISTS slug       TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS sort_order INT  NOT NULL DEFAULT 0;

-- Засеять стандартные категории со стабильными UUID (идемпотентно)
INSERT INTO file_categories (id, name, slug, sort_order) VALUES
  ('a1000001-0000-0000-0000-000000000001', 'Книги',   'books', 1),
  ('a1000001-0000-0000-0000-000000000002', 'Учёба',   'study', 2),
  ('a1000001-0000-0000-0000-000000000003', 'Работа',  'work',  3),
  ('a1000001-0000-0000-0000-000000000004', 'Заметки', 'notes', 4),
  ('a1000001-0000-0000-0000-000000000005', 'Другое',  'other', 5)
ON CONFLICT (id) DO UPDATE
  SET name       = EXCLUDED.name,
      slug       = EXCLUDED.slug,
      sort_order = EXCLUDED.sort_order;
