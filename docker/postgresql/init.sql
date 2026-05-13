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

-- Object metadata E2E fixtures (v0.90.0 Task 7).
-- These objects test the ResolveObject → enrichment → rule → public surface path.

-- Confirmed: composite type in app schema (also creates ambiguous duplicate below).
CREATE TYPE app.address AS (city TEXT, zip TEXT);

-- Confirmed: domain in app schema.
CREATE DOMAIN app.email_address AS TEXT CHECK (VALUE ~* '^[^@]+@[^@]+\.[^@]+$');

-- Confirmed: enum type in app schema.
CREATE TYPE app.color AS ENUM ('red', 'green', 'blue');

-- Confirmed: extension (pgcrypto is bundled in postgres:17 contrib).
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Confirmed: sequence in app schema.
CREATE SEQUENCE app.ticket_seq START WITH 100 INCREMENT BY 5;

-- Confirmed: materialized view in app schema.
CREATE MATERIALIZED VIEW app.user_summary AS SELECT id, name FROM app.users;

-- Confirmed: foreign server + user mapping (tests no-leak of options).
CREATE EXTENSION IF NOT EXISTS postgres_fdw;
CREATE SERVER fs_test FOREIGN DATA WRAPPER postgres_fdw OPTIONS (host 'dummy-host', port '5432', dbname 'remote');
CREATE USER MAPPING FOR root SERVER fs_test OPTIONS (user 'remote_user', password 'secret_should_not_leak');

-- Confirmed: publication (PostgreSQL 17 supports FOR ALL TABLES without superuser).
CREATE PUBLICATION e2e_test_pub FOR ALL TABLES;

-- Confirmed: event trigger (requires superuser; skip if not available).
-- Using comment on a table to test annotation target resolution.
COMMENT ON TABLE app.users IS 'sensitive comment text should not leak';

-- Ambiguous: same type name in two schemas.
CREATE TYPE archive.address AS (street TEXT, city TEXT);
