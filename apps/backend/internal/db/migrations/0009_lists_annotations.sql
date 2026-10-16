-- Each user's favourite series and the ones they want to read.
CREATE TABLE series_lists (
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    series_id INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    list      TEXT NOT NULL CHECK (list IN ('favorites', 'want')),
    added_at  TEXT NOT NULL,
    PRIMARY KEY (user_id, series_id, list)
);

-- Bookmarks and highlights a user made in a book, each with an optional
-- note. Comics only have bookmarks, on a page.
CREATE TABLE annotations (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    book_id    INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('bookmark', 'highlight')),
    -- The page of a comic, or the percentage read of an EPUB.
    page       INTEGER NOT NULL DEFAULT 0,
    -- The EPUB CFI: a point for bookmarks, a range for highlights.
    position   TEXT NOT NULL DEFAULT '',
    -- The highlighted text, or the start of the page for a bookmark.
    text       TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',
    color      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX annotations_by_book ON annotations (user_id, book_id);
