BEGIN;
DROP TABLE sale_line_batches;
ALTER TABLE sales DROP COLUMN prescription_reference, DROP COLUMN pharmacist_verified_by;
ALTER TABLE pharmacy_batch_movements DROP CONSTRAINT pharmacy_batch_movements_kind_check;
ALTER TABLE pharmacy_batch_movements ADD CONSTRAINT pharmacy_batch_movements_kind_check
 CHECK(kind IN ('RECEIPT','QUARANTINE_RELEASE_OUT','QUARANTINE_RELEASE_IN','DISPOSE'));
COMMIT;
