ALTER TABLE app.add_duplicate ADD COLUMN id integer;
ALTER TABLE app.add_system ADD COLUMN IF NOT EXISTS ctid integer;
ALTER TABLE app.rename_missing RENAME COLUMN "ID" TO renamed;
ALTER TABLE app.rename_conflict RENAME COLUMN c TO id;
ALTER TABLE app.rename_same RENAME COLUMN c TO c;
ALTER TABLE app.rename_system RENAME COLUMN ctid TO renamed;
