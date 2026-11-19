ALTER TABLE app.missing_t ALTER COLUMN "ID" TYPE bigint;
ALTER TABLE app.system_t ALTER COLUMN ctid TYPE integer;
ALTER TABLE app.generated_t ALTER COLUMN stored TYPE integer USING stored;
ALTER TABLE app.ordinary_t ALTER COLUMN id TYPE bigint USING id;
