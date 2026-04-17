CREATE TABLE orders (
  id bigint PRIMARY KEY,
  user_id bigint REFERENCES users(id),
  amount numeric NOT NULL CHECK (amount >= 0),
  CONSTRAINT uniq_orders_user UNIQUE (user_id),
  CONSTRAINT chk_orders_amount CHECK (amount >= 0)
);
