ALTER TABLE app.ff SET (fillfactor=9);
ALTER TABLE app.good SET (fillfactor=100, autovacuum_enabled=off);
ALTER TABLE app.opaque SET (fillfactor=8e1);
ALTER TABLE app.reset_bad RESET (private_key='private_payload');
ALTER TABLE app.reset_good RESET (unknown_option, fillfactor, fillfactor);
