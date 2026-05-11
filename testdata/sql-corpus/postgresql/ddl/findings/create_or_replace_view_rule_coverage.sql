CREATE OR REPLACE VIEW active_users AS SELECT id, email FROM users WHERE active = true;
