ALTER TABLE sbory ADD COLUMN IF NOT EXISTS city TEXT NOT NULL DEFAULT 'Алматы';
CREATE INDEX IF NOT EXISTS idx_sbory_city ON sbory(city);
-- Проставляем город для существующих записей (все seed-сборы из Алматы)
UPDATE sbory SET city = 'Алматы' WHERE city = 'Алматы';
