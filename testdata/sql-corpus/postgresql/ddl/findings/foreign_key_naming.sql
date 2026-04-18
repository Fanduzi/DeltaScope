CREATE TABLE fk_names (
  id BIGINT PRIMARY KEY,
  parent_id BIGINT NOT NULL,
  CONSTRAINT badfk FOREIGN KEY (parent_id) REFERENCES parents(id)
);
