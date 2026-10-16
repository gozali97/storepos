BEGIN;
ALTER TABLE pharmacy_batch_movements DROP CONSTRAINT pharmacy_batch_movements_kind_check;
ALTER TABLE pharmacy_batch_movements ADD CONSTRAINT pharmacy_batch_movements_kind_check
 CHECK(kind IN ('RECEIPT','QUARANTINE_RELEASE_OUT','QUARANTINE_RELEASE_IN','DISPOSE','SALE','SALE_VOID','RETURN_QUARANTINE','COUNT_ADJUSTMENT'));
COMMIT;
