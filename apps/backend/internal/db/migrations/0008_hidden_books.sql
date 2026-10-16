-- Hidden books are only shown to admins. Scans never change it.
ALTER TABLE books ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0;
