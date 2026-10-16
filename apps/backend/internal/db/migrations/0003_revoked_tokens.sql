-- Tokens that were logged out before they expired. Rows are only needed
-- until the token would expire anyway, then they get cleaned up.
CREATE TABLE revoked_tokens (
    id         TEXT PRIMARY KEY,
    expires_at TEXT NOT NULL
);
