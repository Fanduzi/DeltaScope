-- PostgreSQL E2E fixture: mirrors the MySQL/TiDB test data for cross-dialect parity.

CREATE SCHEMA IF NOT EXISTS app;
CREATE SCHEMA IF NOT EXISTS archive;

SET search_path TO app;

CREATE TABLE app.users (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  updated_at TIMESTAMP NOT NULL DEFAULT now()
);

INSERT INTO app.users (name) VALUES ('delta'), ('scope');

CREATE TABLE app.orders (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  updated_at TIMESTAMP NOT NULL DEFAULT now()
);

INSERT INTO app.orders (user_id) VALUES (1), (2);

CREATE TABLE app.accounts (
  id BIGSERIAL PRIMARY KEY,
  email VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  updated_at TIMESTAMP NOT NULL DEFAULT now()
);

INSERT INTO app.accounts (email) VALUES ('a@example.com'), ('b@example.com');

-- Constraint for DROP CONSTRAINT → primary key mapping test.
ALTER TABLE app.accounts ADD CONSTRAINT accounts_email_key UNIQUE (email);

-- Index for rename/drop index owner resolution tests.
CREATE INDEX idx_accounts_email ON app.accounts (email);

SET search_path TO archive;

CREATE TABLE archive.users (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  updated_at TIMESTAMP NOT NULL DEFAULT now()
);

INSERT INTO archive.users (name) VALUES ('old-delta');
