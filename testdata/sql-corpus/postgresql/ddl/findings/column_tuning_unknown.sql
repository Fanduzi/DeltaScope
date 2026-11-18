ALTER TABLE app.no_catalog_t ALTER COLUMN id SET STATISTICS 10;
ALTER TABLE app.extension_t ALTER COLUMN id SET(private_unknown_key=1);
ALTER TABLE app.opaque_t ALTER COLUMN id SET(n_distinct='private_quoted_value');
