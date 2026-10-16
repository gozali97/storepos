BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM restaurant_stock_reservations WHERE status='active') THEN RAISE EXCEPTION 'release active reservations before rollback'; END IF; END $$;
DROP TABLE restaurant_stock_reservations;
ALTER TABLE inventory_balances DROP COLUMN reserved_milli;
COMMIT;
