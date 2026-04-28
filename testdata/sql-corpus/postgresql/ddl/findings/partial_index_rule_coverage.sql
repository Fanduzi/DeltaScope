CREATE INDEX idx_users_active_email ON users (email) WHERE active = true;
