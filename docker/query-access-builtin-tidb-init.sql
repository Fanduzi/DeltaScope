CREATE DATABASE IF NOT EXISTS app;

DROP TABLE IF EXISTS app.builtin_semantic_facts;

CREATE TABLE app.builtin_semantic_facts (
  id INT NOT NULL,
  dept INT NOT NULL,
  amount INT NOT NULL,
  name VARCHAR(100) NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);

INSERT INTO app.builtin_semantic_facts (id, dept, amount, name) VALUES
  (1, 10, 100, 'alice'),
  (2, 10, 50, 'bob'),
  (3, 20, 80, 'charlie'),
  (4, 20, 20, 'dave');
