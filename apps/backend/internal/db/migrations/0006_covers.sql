-- Covers an admin uploaded. They replace the one made from the file.
CREATE TABLE book_covers (
    book_id    INTEGER PRIMARY KEY REFERENCES books(id) ON DELETE CASCADE,
    data       BLOB NOT NULL,
    updated_at TEXT NOT NULL
);

-- Covers made from the book files (first page, or an EPUB's cover image),
-- so each is only made once. file_key says which version of the file it was
-- made from; a changed file gets a new cover. data is NULL when the file has
-- no usable cover, so we don't try again on every request.
CREATE TABLE cover_cache (
    book_id  INTEGER PRIMARY KEY REFERENCES books(id) ON DELETE CASCADE,
    file_key TEXT NOT NULL,
    data     BLOB
);
