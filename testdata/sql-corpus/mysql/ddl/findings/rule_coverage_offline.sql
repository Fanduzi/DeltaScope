CREATE TABLE `select` (
  `select` INT,
  id INT,
  very_long_column_name_over_sixty_four_characters_for_rule_coverage_0001 VARCHAR(16),
  bad_varchar VARCHAR(32),
  bad_float DOUBLE,
  bad_text TEXT,
  bad_json JSON,
  bad_bit BIT(1),
  bad_ts TIMESTAMP,
  bad_char CHAR(4),
  bad_charset VARCHAR(16) CHARACTER SET latin1,
  bad_collation VARCHAR(16) COLLATE latin1_swedish_ci,
  no_default INT NOT NULL,
  nullable_col INT DEFAULT 0,
  CONSTRAINT badpk PRIMARY KEY (id, bad_varchar),
  CONSTRAINT baduk UNIQUE KEY (bad_varchar),
  CONSTRAINT badfk FOREIGN KEY (id) REFERENCES parent(id),
  CONSTRAINT badcheck CHECK (id > 0),
  UNIQUE KEY baduniq (bad_varchar),
  KEY badidx (bad_varchar, no_default),
  KEY baddup (bad_varchar),
  KEY badleft (bad_varchar),
  KEY `select` (nullable_col),
  FULLTEXT KEY badfull (bad_text)
) ENGINE=MyISAM DEFAULT CHARSET=latin1 COLLATE=latin1_swedish_ci ROW_FORMAT=COMPACT AUTO_INCREMENT=9 COMMENT='comment too long';

CREATE TABLE no_pk (id INT);
CREATE TABLE bad_pk_semantics (
  id INT PRIMARY KEY
) COMMENT='pk semantics';
CREATE TABLE very_long_table_name_over_sixty_four_characters_for_rule_coverage_0001 (id INT PRIMARY KEY);
CREATE TABLE like_copy LIKE no_pk;
CREATE TABLE as_copy AS SELECT * FROM no_pk;
CREATE TABLE part_table (id INT PRIMARY KEY) PARTITION BY HASH(id) PARTITIONS 2;
CREATE VIEW v_bad AS SELECT id FROM no_pk;
DROP VIEW v_bad;
DROP TABLE old_table;
TRUNCATE TABLE old_table;

ALTER TABLE no_pk DROP COLUMN id;
ALTER TABLE no_pk DROP PRIMARY KEY;
ALTER TABLE no_pk DROP INDEX badidx;
ALTER TABLE no_pk RENAME TO no_pk_new;
ALTER TABLE no_pk RENAME COLUMN id TO id2;
ALTER TABLE no_pk RENAME INDEX badidx TO good_idx;
ALTER TABLE no_pk ADD INDEX badadd (id, id2);
ALTER TABLE no_pk ADD INDEX badadddup (id);
ALTER TABLE no_pk ADD INDEX dupa (id), ADD INDEX dupb (id);
ALTER TABLE no_pk ADD INDEX leftwide (id, id2), ADD INDEX leftnarrow (id);
ALTER TABLE no_pk ADD UNIQUE KEY badadduniq (id);
ALTER TABLE no_pk ADD UNIQUE KEY badadduq2 (id);
ALTER TABLE no_pk ADD UNIQUE KEY uniq_overlap_a (id), ADD UNIQUE KEY uniq_overlap_b (id);
ALTER TABLE no_pk ADD UNIQUE KEY uniq_wide (id, id2), ADD UNIQUE KEY uniq_narrow (id);
ALTER TABLE no_pk ADD FULLTEXT KEY badaddfull (id2);
ALTER TABLE no_pk CHANGE COLUMN id id2 JSON NOT NULL AUTO_INCREMENT DEFAULT 1;
ALTER TABLE no_pk MODIFY COLUMN id JSON NOT NULL AUTO_INCREMENT DEFAULT 1;
ALTER TABLE no_pk ENGINE=InnoDB AUTO_INCREMENT=1;
ALTER TABLE no_pk ADD COLUMN existing INT;
ALTER TABLE no_pk ADD COLUMN other_col INT;
CREATE DATABASE app;
DROP DATABASE app;
