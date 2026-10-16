-- What happened: logins, password changes and admin actions. Entries keep
-- the username, so they still make sense after the user is deleted.
CREATE TABLE activity_log (
    id          INTEGER PRIMARY KEY,
    at          TEXT NOT NULL,
    event       TEXT NOT NULL,
    user_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    username    TEXT NOT NULL DEFAULT '',
    ip          TEXT NOT NULL DEFAULT '',
    device_kind TEXT NOT NULL DEFAULT '',
    device      TEXT NOT NULL DEFAULT '',
    detail      TEXT NOT NULL DEFAULT ''
);

CREATE INDEX activity_log_by_time ON activity_log (at);

-- Who is reading what right now, one row per user and browser or app
-- (client, its User-Agent). Readers report every minute while open.
CREATE TABLE reading_sessions (
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client      TEXT NOT NULL,
    book_id     INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    ip          TEXT NOT NULL DEFAULT '',
    device_kind TEXT NOT NULL DEFAULT '',
    device      TEXT NOT NULL DEFAULT '',
    started_at  TEXT NOT NULL,
    last_seen   TEXT NOT NULL,
    PRIMARY KEY (user_id, client)
);
