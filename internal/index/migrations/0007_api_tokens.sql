-- Long-lived API / MCP tokens. Only a SHA-256 hash is stored; the plaintext is
-- shown once at creation. `prefix` is a short displayable fragment.
CREATE TABLE IF NOT EXISTS api_tokens (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    prefix     TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created    TEXT NOT NULL,
    last_used  TEXT,
    expires    TEXT
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_hash ON api_tokens(token_hash);
