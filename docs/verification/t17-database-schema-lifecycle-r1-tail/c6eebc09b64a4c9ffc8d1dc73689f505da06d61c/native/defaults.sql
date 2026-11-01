SELECT VERSION();
SHOW SESSION VARIABLES WHERE Variable_name IN
  ('character_set_server', 'collation_server', 'character_set_database', 'default_collation_for_utf8mb4');
CREATE DATABASE ds17tail_create CHARACTER SET utf8mb4;
CREATE DATABASE ds17tail_alter CHARACTER SET latin1;
ALTER DATABASE ds17tail_alter CHARACTER SET utf8mb4;
SELECT SCHEMA_NAME, DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME
FROM information_schema.SCHEMATA
WHERE SCHEMA_NAME IN ('ds17tail_create', 'ds17tail_alter')
ORDER BY SCHEMA_NAME;
SET SESSION default_collation_for_utf8mb4 = 'utf8mb4_general_ci';
SHOW SESSION VARIABLES WHERE Variable_name = 'default_collation_for_utf8mb4';
CREATE DATABASE ds17tail_general CHARACTER SET utf8mb4;
ALTER DATABASE ds17tail_alter CHARACTER SET utf8mb4;
SELECT SCHEMA_NAME, DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME
FROM information_schema.SCHEMATA
WHERE SCHEMA_NAME IN ('ds17tail_general', 'ds17tail_alter')
ORDER BY SCHEMA_NAME;
SET SESSION character_set_server = 'latin1';
SET SESSION collation_server = 'latin1_bin';
SHOW SESSION VARIABLES WHERE Variable_name IN ('character_set_server', 'collation_server');
CREATE DATABASE ds17tail_server;
SELECT SCHEMA_NAME, DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME
FROM information_schema.SCHEMATA
WHERE SCHEMA_NAME = 'ds17tail_server';
