-- T17 database/schema lifecycle rules corpus (offline audit): conditional
-- clauses prove the schema state for the next statement — IF NOT EXISTS over
-- an unvouched schema proves presence, IF EXISTS proves absence.
CREATE DATABASE IF NOT EXISTS app;
CREATE DATABASE app;
DROP DATABASE IF EXISTS gone;
DROP DATABASE gone;
ALTER DATABASE gone CHARACTER SET utf8mb4;
DROP DATABASE mysql;
ALTER DATABASE mysql CHARACTER SET utf8mb4;
ALTER DATABASE blocked CHARACTER SET utf8mb4;
DROP DATABASE blocked;
ALTER DATABASE t17enc DEFAULT ENCRYPTION='Y';
