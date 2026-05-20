ALTER TABLE users ALTER COLUMN email RESET (n_distinct);
ALTER TABLE users ALTER COLUMN email RESET (n_distinct, n_distinct_inherited);
