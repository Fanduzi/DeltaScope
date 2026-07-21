-- MySQL TLS E2E fixture: minimal schema for testing TLS connections.

CREATE TABLE IF NOT EXISTS app.users (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(255) NOT NULL DEFAULT ''
);

INSERT INTO app.users (name) VALUES ('tls-test-user');
