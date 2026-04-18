DELETE FROM users;
UPDATE users SET status = 'disabled' ORDER BY id LIMIT 1;
UPDATE users u JOIN orders o SET u.status = 'disabled';
DELETE FROM users WHERE id IN (SELECT id FROM archived_users);
INSERT INTO users(id) VALUES (1), (2);
REPLACE INTO users(id) VALUES (1);
INSERT INTO users(id) SELECT id FROM archived_users;
INSERT INTO users(id) VALUES (1) ON DUPLICATE KEY UPDATE id = VALUES(id);
