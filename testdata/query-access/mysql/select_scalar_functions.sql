SELECT LOWER(name), UPPER(email), LENGTH(name), CHAR_LENGTH(name), ABS(amount), CEIL(amount), FLOOR(amount), COALESCE(name, email), NULLIF(name, email), IFNULL(name, 'unknown') FROM app.users;
