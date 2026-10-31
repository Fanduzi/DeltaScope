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
-- T17-R1: the TiDB placement binding premise is unvouchable, so this
-- IF NOT EXISTS create arm cannot converge — pg stays unvouched and the
-- successor statement must not see a fabricated presence.
CREATE DATABASE IF NOT EXISTS pg PLACEMENT POLICY=p1;
CREATE DATABASE pg;
-- T17-R1: tiflash_replica parses into UnextractedOptions; on TiDB its
-- outcome is unproven — t17tf stays unvouched, never a fabricated
-- present with batchCreated facts.
CREATE DATABASE t17tf SET TIFLASH REPLICA 1;
CREATE TABLE t17tf.t (id INT);
