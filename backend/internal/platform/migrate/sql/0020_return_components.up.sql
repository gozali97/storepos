BEGIN;
ALTER TABLE sale_return_lines ADD COLUMN net_cents BIGINT, ADD COLUMN tax_cents BIGINT, ADD COLUMN service_charge_cents BIGINT;
ALTER TABLE sale_returns ADD COLUMN cash_rounding_cents BIGINT;
-- Existing records remain NULL: missing historical allocation must be visible in reports.
CREATE INDEX sales_report_period ON sales(business_id,branch_id,completed_at);
CREATE INDEX returns_report_period ON sale_returns(business_id,branch_id,created_at);
COMMIT;
