BEGIN;
ALTER TABLE reconciliation_findings DROP CONSTRAINT reconciliation_findings_kind_check;
ALTER TABLE reconciliation_findings ADD CONSTRAINT reconciliation_findings_kind_check CHECK(kind IN ('stock','payment','reservation'));
CREATE INDEX restaurant_active_reservations ON restaurant_stock_reservations(business_id,branch_id,variant_id) WHERE status='active';
COMMIT;
