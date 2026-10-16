BEGIN;
DROP INDEX returns_report_period;
DROP INDEX sales_report_period;
ALTER TABLE sale_returns DROP COLUMN cash_rounding_cents;
ALTER TABLE sale_return_lines DROP COLUMN net_cents,DROP COLUMN tax_cents,DROP COLUMN service_charge_cents;
COMMIT;
