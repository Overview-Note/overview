ALTER TABLE notes ADD COLUMN path_lower  TEXT NOT NULL DEFAULT '';
ALTER TABLE notes ADD COLUMN title_lower TEXT NOT NULL DEFAULT '';
ALTER TABLE notes ADD COLUMN name_lower  TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_notes_path_lower  ON notes (path_lower);
CREATE INDEX IF NOT EXISTS idx_notes_title_lower ON notes (title_lower);
CREATE INDEX IF NOT EXISTS idx_notes_name_lower  ON notes (name_lower);
