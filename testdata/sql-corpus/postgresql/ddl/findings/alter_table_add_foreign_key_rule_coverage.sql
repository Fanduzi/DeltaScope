ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE public.orders ADD CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES auth.users(id);
