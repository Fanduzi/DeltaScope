CREATE INDEX idx_users_email_cover ON users (email) INCLUDE (name, active);
