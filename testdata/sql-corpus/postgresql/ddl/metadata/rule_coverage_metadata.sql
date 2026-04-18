ALTER TABLE missing_table ADD COLUMN c INTEGER;
ALTER TABLE users ADD CONSTRAINT existing_idx UNIQUE (existing);
ALTER TABLE users ADD COLUMN existing INTEGER;
ALTER TABLE users RENAME COLUMN missing_col TO renamed_col;
DROP TABLE missing_table;
DROP TABLE big_table;
TRUNCATE TABLE missing_table;
TRUNCATE TABLE big_table;
