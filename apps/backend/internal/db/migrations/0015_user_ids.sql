-- The highest user ID ever given out. SQLite would give a deleted user's ID
-- to the next new user, and that user's old login tokens with it.
CREATE TABLE id_counters (
    name TEXT PRIMARY KEY,
    last INTEGER NOT NULL
);

INSERT INTO id_counters (name, last) SELECT 'users', COALESCE(MAX(id), 0) FROM users;
