-- Books users asked for, one per ISBN, with what the lookup found.
CREATE TABLE book_requests (
    id         INTEGER PRIMARY KEY,
    isbn       TEXT NOT NULL UNIQUE,
    title      TEXT NOT NULL,
    authors    TEXT NOT NULL DEFAULT '',
    published  TEXT NOT NULL DEFAULT '',
    cover_url  TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'added', 'declined')),
    -- The book an admin added for it.
    book_id    INTEGER REFERENCES books(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Who asked for each book; once per user.
CREATE TABLE book_request_votes (
    request_id INTEGER NOT NULL REFERENCES book_requests(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    PRIMARY KEY (request_id, user_id)
);
