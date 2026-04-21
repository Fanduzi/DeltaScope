ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0);
