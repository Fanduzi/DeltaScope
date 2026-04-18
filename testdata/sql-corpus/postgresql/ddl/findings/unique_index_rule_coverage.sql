CREATE INDEX bad_users_email ON users (email);
CREATE UNIQUE INDEX bad_users_email_unique ON users (email);
CREATE INDEX idx_users_email_tenant ON users (email, tenant_id);
