BEGIN;
DROP INDEX restaurant_active_reservations;
-- Refuse rollback if reservation findings exist rather than deleting audit evidence.
ALTER TABLE reconciliation_findings DROP CONSTRAINT reconciliation_findings_kind_check;
ALTER TABLE reconciliation_findings ADD CONSTRAINT reconciliation_findings_kind_check CHECK(kind IN ('stock','payment'));
COMMIT;
