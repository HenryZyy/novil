-- A user took the book out of Continue reading. Their progress stays;
-- reading on brings the book back.
ALTER TABLE progress ADD COLUMN dismissed INTEGER NOT NULL DEFAULT 0;
