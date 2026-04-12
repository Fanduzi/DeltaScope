CREATE TABLE orders (
  id bigint PRIMARY KEY,
  user_id bigint REFERENCES users(id),
  amount numeric CHECK (amount > 0),
  external_id text UNIQUE,
  CONSTRAINT orders_amount_check CHECK (amount < 100000),
  CONSTRAINT orders_ref_unique UNIQUE (external_id),
  CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id)
);
