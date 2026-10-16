-- Rollback 0004.
BEGIN;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS sale_lines;
DROP TABLE IF EXISTS sales;
DROP TABLE IF EXISTS shifts;
DROP TABLE IF EXISTS stock_movements;
DROP FUNCTION IF EXISTS stock_movements_block_mutation();
DROP TABLE IF EXISTS inventory_balances;
DROP INDEX IF EXISTS registers_business_id_id_key;
COMMIT;
