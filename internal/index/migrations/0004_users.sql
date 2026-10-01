CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'member',
    created       TEXT NOT NULL DEFAULT '',
    updated       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS sessions (
    token   TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    created TEXT NOT NULL DEFAULT '',
    expires TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions (user_id);
