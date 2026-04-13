CREATE TABLE orders (
  id bigint PRIMARY KEY,
  user_id bigint REFERENCES public.users(id),
  approver_id bigint,
  CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES public.users(id)
);
