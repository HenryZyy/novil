-- A requested book's cover, downloaded once from where the lookup found
-- it, so it isn't fetched from the book databases on every view.
ALTER TABLE book_requests ADD COLUMN cover BLOB;

-- What ISBN lookups found (info is the isbn.Info as JSON), so the same ISBN
-- isn't looked up again for a while. found = 0 remembers "no such book".
CREATE TABLE isbn_lookups (
    isbn       TEXT PRIMARY KEY,
    found      INTEGER NOT NULL,
    info       TEXT NOT NULL,
    fetched_at TEXT NOT NULL
);
