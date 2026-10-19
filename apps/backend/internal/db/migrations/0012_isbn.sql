-- A book's ISBN-13: from the file (EPUB identifier, ComicInfo GTIN), or as
-- an admin set it, like the other metadata.
ALTER TABLE books ADD COLUMN isbn TEXT NOT NULL DEFAULT '';
ALTER TABLE book_overrides ADD COLUMN isbn TEXT;
