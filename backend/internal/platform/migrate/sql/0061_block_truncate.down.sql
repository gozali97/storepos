BEGIN;
DROP TRIGGER pharmacy_movements_no_truncate ON pharmacy_batch_movements;
DROP TRIGGER stock_movements_no_truncate ON stock_movements;
DROP TRIGGER audit_logs_no_truncate ON audit_logs;
COMMIT;
