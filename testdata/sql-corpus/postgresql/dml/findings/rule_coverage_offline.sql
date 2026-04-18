DELETE FROM users;
DELETE FROM users WHERE id IN (SELECT id FROM archived_users);
INSERT INTO users(id) VALUES (1), (2);
INSERT INTO users(id) SELECT id FROM archived_users;
