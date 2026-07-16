SELECT row_number() OVER (PARTITION BY length(name) ORDER BY id) FROM users
