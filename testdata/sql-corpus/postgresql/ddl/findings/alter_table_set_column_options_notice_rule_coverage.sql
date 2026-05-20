ALTER TABLE users ALTER COLUMN email SET (n_distinct = 1);
ALTER TABLE users ALTER COLUMN email SET (n_distinct = 1, n_distinct_inherited = -1);
