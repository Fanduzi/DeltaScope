-- PostgreSQL TLS E2E fixture: minimal schema for testing TLS connections.

CREATE SCHEMA IF NOT EXISTS app;

CREATE TABLE app.users (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL DEFAULT ''
);

INSERT INTO app.users (name) VALUES ('tls-test-user');
