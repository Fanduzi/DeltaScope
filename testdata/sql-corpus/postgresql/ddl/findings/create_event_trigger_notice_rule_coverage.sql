CREATE EVENT TRIGGER trg_ddl ON ddl_command_end EXECUTE FUNCTION log_ddl()
