ALTER TABLE missing_table ADD COLUMN c INT;
ALTER TABLE users ADD COLUMN existing INT;
ALTER TABLE users_dc DROP COLUMN missing_col;
ALTER TABLE users_mc MODIFY COLUMN missing_col BIGINT;
ALTER TABLE users_cc CHANGE COLUMN missing_col renamed_col BIGINT;
ALTER TABLE users_rc RENAME COLUMN missing_col TO renamed_col;
ALTER TABLE users_ai ADD INDEX existing_idx (existing);
ALTER TABLE users_di DROP INDEX missing_idx;
ALTER TABLE users_ri RENAME INDEX missing_idx TO renamed_idx;
ALTER TABLE no_pk DROP PRIMARY KEY;
ALTER TABLE compat_table MODIFY COLUMN amount TINYINT NOT NULL DEFAULT 1;
ALTER TABLE compat_table2 CHANGE COLUMN amount amount2 VARCHAR(8) NOT NULL DEFAULT 'x';
ALTER TABLE compat_table3 ENGINE=InnoDB AUTO_INCREMENT=1;
DROP TABLE gone_table;
TRUNCATE TABLE gone_table_2;
DROP TABLE big_table;
TRUNCATE TABLE huge_table;
CREATE TABLE wide_table (
  a VARCHAR(6000),
  b VARCHAR(6000),
  c VARCHAR(6000),
  KEY idx_wide (a, b)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=COMPACT;
CREATE INDEX idx_ci_missing ON ci_table (Missing_Col);
