ALTER TABLE users ALTER COLUMN full_name SET EXPRESSION AS (first_name || ' ' || last_name);
