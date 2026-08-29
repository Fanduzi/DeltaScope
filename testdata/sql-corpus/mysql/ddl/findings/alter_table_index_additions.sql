ALTER TABLE users ADD INDEX idx_email (email);
ALTER TABLE users ADD KEY idx_name (name);
ALTER TABLE users ADD UNIQUE INDEX uniq_email (email);
ALTER TABLE users ADD UNIQUE KEY uniq_name (name);
ALTER TABLE posts ADD FULLTEXT INDEX ft_body (body);
ALTER TABLE users ADD INDEX idx_algorithm (email), ALGORITHM=INPLACE, LOCK=NONE;
