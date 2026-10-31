-- T17 database/schema lifecycle rules corpus (offline audit): conditional
-- clauses prove the schema state for the next statement — IF NOT EXISTS over
-- an unvouched schema proves presence only when the create arm adjudicates
-- Apply for every declared option (T17-R1), IF EXISTS proves absence.
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
-- T17-R1: version 5.7.44 refuses schema ENCRYPTION (probed 1064), so this
-- IF NOT EXISTS create arm cannot converge — encg stays unvouched and the
-- successor statement must not see a fabricated presence.
CREATE DATABASE IF NOT EXISTS encg DEFAULT ENCRYPTION='Y';
CREATE DATABASE encg;
-- T17-R1: tiflash_replica parses through the shared grammar into
-- UnextractedOptions; on MySQL it is a proven vendor refusal — t17tf keeps
-- its unvouched pre-state in this offline fixture; no batchCreated is minted.
CREATE DATABASE t17tf SET TIFLASH REPLICA 1;
CREATE TABLE t17tf.t (id INT);
