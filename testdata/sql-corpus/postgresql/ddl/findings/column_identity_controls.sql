ALTER TABLE IF EXISTS app.absent ADD COLUMN fresh integer;
ALTER TABLE IF EXISTS app.sequence_name ADD COLUMN fresh integer;
ALTER TABLE app.guard_existing ADD COLUMN IF NOT EXISTS id integer NOT NULL;
ALTER TABLE app.fresh ADD COLUMN "ID" integer;
ALTER TABLE app.rename_spaces RENAME COLUMN " " TO " ID ";
