CREATE TABLE "select" (
  id integer,
  "select" integer,
  very_long_column_name_over_sixty_four_characters_for_rule_coverage_0001 varchar(16),
  bad_varchar varchar(32),
  bad_float double precision,
  bad_text text,
  bad_json json,
  bad_bit bit(1),
  bad_ts timestamp,
  bad_char char(4),
  no_default integer NOT NULL,
  nullable_col integer DEFAULT 0,
  CONSTRAINT badpk PRIMARY KEY (id, bad_varchar),
  CONSTRAINT baduk UNIQUE (bad_varchar),
  CONSTRAINT badcheck CHECK (id > 0)
);

CREATE TABLE no_pk (id integer);
CREATE TABLE very_long_table_name_over_sixty_four_characters_for_rule_coverage_0001 (id bigint PRIMARY KEY);
CREATE TABLE like_copy (LIKE no_pk);
CREATE TABLE as_copy AS SELECT 1 AS id;
CREATE VIEW v_bad AS SELECT id FROM no_pk;
DROP VIEW v_bad;
DROP TABLE old_table;
TRUNCATE TABLE old_table;
