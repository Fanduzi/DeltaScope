-- T30 (#109): event definition audit — modeled schedule/completion/
-- enablement attributes, DO body reuse, and the enabled-execution
-- warning under the MySQL dialect. DISABLED bodies still audit.
CREATE EVENT ev_cleanup ON SCHEDULE EVERY 1 DAY ENABLE DO DELETE FROM audit_log;
ALTER EVENT ev_cleanup DISABLE;
DROP EVENT IF EXISTS ev_old;
CREATE EVENT ev_once ON SCHEDULE AT CURRENT_TIMESTAMP DISABLE DO DELETE FROM audit_log WHERE id = 1;
