DROP TABLE harmless, sensitive;
RENAME TABLE harmless TO harmless_old, sensitive TO sensitive_old;
RENAME TABLE harmless TO harmless_old, other TO sensitive;
ALTER TABLE harmless RENAME TO sensitive;
ALTER TABLE app.harmless RENAME TO sensitive;
DROP TABLE `a.b`.`c`, `a`.`b.c`;
DROP TABLE harmless, other;
