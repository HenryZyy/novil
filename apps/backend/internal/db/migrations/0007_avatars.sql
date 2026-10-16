-- Profile pictures, one per user, stored on the server so every device
-- shows the same one.
CREATE TABLE user_avatars (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    data    BLOB NOT NULL
);

-- When the picture last changed, NULL without one. Kept on users so user
-- lists know who has a picture without loading the pictures.
ALTER TABLE users ADD COLUMN avatar_updated_at TEXT;
