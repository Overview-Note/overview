CREATE TABLE IF NOT EXISTS folder_changes (
  path TEXT PRIMARY KEY, changed_seq INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_folder_changes_changed_seq ON folder_changes(changed_seq);
