CREATE TABLE public.orders (
  id bigint PRIMARY KEY,
  approver_id bigint REFERENCES users(id)
);
