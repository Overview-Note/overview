CREATE TABLE IF NOT EXISTS notes (
    id      TEXT PRIMARY KEY,
    path    TEXT NOT NULL UNIQUE,
    title   TEXT NOT NULL DEFAULT '',
    tags    TEXT NOT NULL DEFAULT '',
    body    TEXT NOT NULL DEFAULT '',
    created TEXT NOT NULL DEFAULT '',
    updated TEXT NOT NULL DEFAULT '',
    size    INTEGER NOT NULL DEFAULT 0,
    version TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_notes_path ON notes (path);

CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
    id UNINDEXED,
    title,
    tags,
    body,
    tokenize = 'unicode61'
);
