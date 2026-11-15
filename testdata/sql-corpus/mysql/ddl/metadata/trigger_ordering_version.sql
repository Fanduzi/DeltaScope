-- T29 (#108): FOLLOWS/PRECEDES ordering is a MySQL 5.7.2+ feature —
-- on a 5.7.0 metadata target the ordering rule is a determined
-- negative while the body still audits.
CREATE TRIGGER tr_ord BEFORE INSERT ON t FOR EACH ROW FOLLOWS other DELETE FROM audit_log;
