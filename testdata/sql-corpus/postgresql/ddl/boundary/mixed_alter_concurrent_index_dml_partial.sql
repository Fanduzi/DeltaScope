ALTER TABLE users ADD COLUMN bio text NOT NULL;
CREATE INDEX CONCURRENTLY idx_users_bio ON users (bio);
CREATE INDEX idx_broken ON;
DELETE FROM users;
