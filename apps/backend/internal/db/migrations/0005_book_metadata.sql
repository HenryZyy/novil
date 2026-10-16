-- What the book file says about itself. Filled in by scans and uploads;
-- existing books get it on the next scan.
ALTER TABLE books ADD COLUMN author      TEXT NOT NULL DEFAULT '';
ALTER TABLE books ADD COLUMN language    TEXT NOT NULL DEFAULT '';
ALTER TABLE books ADD COLUMN released    TEXT NOT NULL DEFAULT '';
ALTER TABLE books ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- Values an admin set by hand. They win over the file's, and scans never
-- touch them. NULL means "use the file's value".
CREATE TABLE book_overrides (
    book_id     INTEGER PRIMARY KEY REFERENCES books(id) ON DELETE CASCADE,
    title       TEXT,
    author      TEXT,
    language    TEXT,
    released    TEXT,
    description TEXT
);
