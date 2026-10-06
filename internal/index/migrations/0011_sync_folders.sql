CREATE TABLE IF NOT EXISTS folder_tombstones (
  path TEXT PRIMARY KEY, deleted_at TEXT NOT NULL, device TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_folder_tombstones_deleted_at ON folder_tombstones(deleted_at);
