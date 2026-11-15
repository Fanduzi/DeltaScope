-- T28 (#107): routine-body control-flow, variable binding, and
-- dynamic-SQL boundary analysis under the MySQL dialect.
CREATE PROCEDURE p_shadow(IN s INT) BEGIN DECLARE s INT; SELECT 1; END;
CREATE PROCEDURE p_flow() BEGIN IF 1=1 THEN DELETE FROM t; END IF; END;
CREATE PROCEDURE p_dyn(IN v TEXT) BEGIN SET @q=v; PREPARE stmt FROM @q; EXECUTE stmt; END;
