ALTER TABLE orders ALTER CONSTRAINT orders_user_id_fkey DEFERRABLE;
ALTER TABLE payments ALTER CONSTRAINT payments_order_id_fkey DEFERRABLE;
