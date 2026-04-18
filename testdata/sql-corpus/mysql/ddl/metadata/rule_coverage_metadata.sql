ALTER TABLE missing_table ADD COLUMN c INT;
ALTER TABLE users ADD COLUMN existing INT;
ALTER TABLE users DROP COLUMN missing_col;
ALTER TABLE users MODIFY COLUMN missing_col BIGINT;
ALTER TABLE users CHANGE COLUMN missing_col renamed_col BIGINT;
ALTER TABLE users RENAME COLUMN missing_col TO renamed_col;
ALTER TABLE users ADD INDEX existing_idx (existing);
ALTER TABLE users DROP INDEX missing_idx;
ALTER TABLE users RENAME INDEX missing_idx TO renamed_idx;
ALTER TABLE no_pk DROP PRIMARY KEY;
ALTER TABLE compat_table MODIFY COLUMN amount TINYINT NOT NULL DEFAULT 1;
ALTER TABLE compat_table CHANGE COLUMN amount amount2 VARCHAR(8) NOT NULL DEFAULT 'x';
ALTER TABLE compat_table ENGINE=InnoDB AUTO_INCREMENT=1;
DROP TABLE missing_table;
DROP TABLE big_table;
TRUNCATE TABLE missing_table;
TRUNCATE TABLE big_table;
CREATE TABLE wide_table (
  a VARCHAR(6000),
  b VARCHAR(6000),
  c VARCHAR(6000),
  KEY idx_wide (a, b)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=COMPACT;
