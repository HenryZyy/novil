CREATE TABLE libraries (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    path       TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE series (
    id         INTEGER PRIMARY KEY,
    library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    path       TEXT NOT NULL,
    UNIQUE (library_id, path)
);

CREATE TABLE books (
    id         INTEGER PRIMARY KEY,
    series_id  INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    path       TEXT NOT NULL UNIQUE,
    format     TEXT NOT NULL,
    size       INTEGER NOT NULL,
    mod_time   TEXT NOT NULL,
    page_count INTEGER NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX books_series_id ON books(series_id);

CREATE TABLE progress (
    book_id    INTEGER PRIMARY KEY REFERENCES books(id) ON DELETE CASCADE,
    page       INTEGER NOT NULL,
    completed  INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);
