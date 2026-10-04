-- Email-based user management: accounts may carry an email address and a
-- lifecycle status. Invited users are created with an empty password hash
-- (bcrypt will always reject an empty hash) so they cannot log in until they
-- accept the invitation. `email` stores a lower-cased, normalised value.
ALTER TABLE users ADD COLUMN email TEXT;
ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE users ADD COLUMN email_verified INTEGER NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email
    ON users(email) WHERE email IS NOT NULL AND email <> '';
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);

-- Single-use, purpose-scoped tokens for invitations, password resets and email
-- verification. Only a SHA-256 hash of the plaintext token is stored.
CREATE TABLE IF NOT EXISTS user_tokens (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    purpose    TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created    TEXT NOT NULL,
    expires    TEXT NOT NULL,
    used       INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_user_tokens_hash ON user_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_user_tokens_user ON user_tokens(user_id, purpose);
