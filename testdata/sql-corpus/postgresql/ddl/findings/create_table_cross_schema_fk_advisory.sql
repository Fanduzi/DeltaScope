CREATE TABLE public.orders (
  id bigint PRIMARY KEY,
  approver_id bigint,
  CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES auth.users(id)
);
