CREATE DATABASE IF NOT EXISTS app;

DROP TABLE IF EXISTS app.builtin_semantic_facts;

CREATE TABLE app.builtin_semantic_facts (
  id INT NOT NULL,
  dept INT NOT NULL,
  amount INT NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB;

INSERT INTO app.builtin_semantic_facts (id, dept, amount) VALUES
  (1, 10, 100),
  (2, 10, 50),
  (3, 20, 80),
  (4, 20, 20);
