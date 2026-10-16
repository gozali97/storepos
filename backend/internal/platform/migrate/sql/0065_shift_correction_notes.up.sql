BEGIN;
-- Catatan koreksi tidak mengubah angka/status shift final (CORE-026).
CREATE TABLE shift_correction_notes (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 shift_id UUID NOT NULL,
 body TEXT NOT NULL CHECK(length(btrim(body))>0 AND length(body)<=1000),
 created_by UUID NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(business_id,shift_id) REFERENCES shifts(business_id,id)
);
CREATE INDEX shift_correction_notes_history ON shift_correction_notes(business_id,shift_id,id);
CREATE TRIGGER shift_notes_no_update BEFORE UPDATE OR DELETE ON shift_correction_notes FOR EACH ROW EXECUTE FUNCTION audit_logs_block_mutation();
CREATE TRIGGER shift_notes_no_truncate BEFORE TRUNCATE ON shift_correction_notes FOR EACH STATEMENT EXECUTE FUNCTION audit_logs_block_mutation();
CREATE INDEX shifts_branch_history ON shifts(business_id,branch_id,opened_at DESC,id DESC);
COMMIT;
