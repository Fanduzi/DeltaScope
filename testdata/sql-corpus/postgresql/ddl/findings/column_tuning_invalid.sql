ALTER TABLE app.stat_t ALTER COLUMN id SET STATISTICS -2;
ALTER TABLE app.opt_t ALTER COLUMN id SET(n_distinct_inherited=-1.1);
ALTER TABLE app.dup_t ALTER COLUMN id SET(n_distinct=1,n_distinct=2);
ALTER TABLE app.missing_t DROP COLUMN ghost;
