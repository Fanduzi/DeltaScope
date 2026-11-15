-- T29 (#108): trigger definition audit — timing/event/target binding,
-- FOR EACH ROW body reuse, and the OLD/NEW pseudo-column boundary
-- under the MySQL dialect.
CREATE TRIGGER tr_audit BEFORE INSERT ON t FOR EACH ROW DELETE FROM audit_log;
DROP TRIGGER IF EXISTS tr_old;
CREATE TRIGGER tr_bad BEFORE INSERT ON t FOR EACH ROW UPDATE t SET x = OLD.y;
