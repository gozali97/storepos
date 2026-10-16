-- Rollback 0005.
BEGIN;
ALTER TABLE sales
    DROP COLUMN IF EXISTS discount_approved_by, DROP COLUMN IF EXISTS discount_reason,
    DROP COLUMN IF EXISTS tax_mode, DROP COLUMN IF EXISTS tax_rate_bps, DROP COLUMN IF EXISTS tax_rule_name;
DROP TABLE IF EXISTS tax_rules;
DROP TRIGGER IF EXISTS shifts_lock_closed ON shifts;
DROP FUNCTION IF EXISTS shifts_lock_closed();
ALTER TABLE shifts
    DROP COLUMN IF EXISTS close_approved_by, DROP COLUMN IF EXISTS closed_by, DROP COLUMN IF EXISTS close_note,
    DROP COLUMN IF EXISTS variance_cents, DROP COLUMN IF EXISTS expected_cash_cents, DROP COLUMN IF EXISTS counted_cash_cents;
DROP TABLE IF EXISTS cash_movements;
COMMIT;
