CREATE TABLE orders (
  id bigint PRIMARY KEY,
  user_id bigint REFERENCES users(id)
);
