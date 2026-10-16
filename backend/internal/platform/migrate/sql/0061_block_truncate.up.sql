BEGIN;
-- Trigger baris tidak menangkap TRUNCATE; tanpa ini seluruh jejak audit/stok bisa dikosongkan sekaligus (CORE-042).
CREATE TRIGGER audit_logs_no_truncate BEFORE TRUNCATE ON audit_logs
    FOR EACH STATEMENT EXECUTE FUNCTION audit_logs_block_mutation();
CREATE TRIGGER stock_movements_no_truncate BEFORE TRUNCATE ON stock_movements
    FOR EACH STATEMENT EXECUTE FUNCTION stock_movements_block_mutation();
CREATE TRIGGER pharmacy_movements_no_truncate BEFORE TRUNCATE ON pharmacy_batch_movements
    FOR EACH STATEMENT EXECUTE FUNCTION stock_movements_block_mutation();
COMMIT;
