CREATE EVENT app.fixture_event ON SCHEDULE EVERY 1 DAY DISABLE DO SET @audit_marker=1;
