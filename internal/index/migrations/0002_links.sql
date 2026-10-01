CREATE TABLE IF NOT EXISTS links (
    source_id   TEXT NOT NULL,
    source_path TEXT NOT NULL,
    target_raw  TEXT NOT NULL,
    target_key  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_links_target_key ON links (target_key);
CREATE INDEX IF NOT EXISTS idx_links_source ON links (source_id);
