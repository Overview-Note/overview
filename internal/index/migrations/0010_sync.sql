ALTER TABLE notes ADD COLUMN changed_seq INTEGER NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS note_tombstones (
  id TEXT PRIMARY KEY, path TEXT NOT NULL, deleted_at TEXT NOT NULL, device TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_tombstones_deleted_at ON note_tombstones(deleted_at);
