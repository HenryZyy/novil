-- When a book was first found, for "recently added".
ALTER TABLE books ADD COLUMN created_at TEXT NOT NULL DEFAULT '1970-01-01T00:00:00Z';
UPDATE books SET created_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now');

-- Where in an EPUB the reader is, as an EPUB CFI. Empty for comics.
ALTER TABLE progress ADD COLUMN position TEXT NOT NULL DEFAULT '';
