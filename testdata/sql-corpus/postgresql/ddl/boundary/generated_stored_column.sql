CREATE TABLE users (
  first_name text,
  last_name text,
  full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED
);
