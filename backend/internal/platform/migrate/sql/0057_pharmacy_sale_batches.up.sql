BEGIN;
ALTER TABLE pharmacy_batch_movements DROP CONSTRAINT pharmacy_batch_movements_kind_check;
ALTER TABLE pharmacy_batch_movements ADD CONSTRAINT pharmacy_batch_movements_kind_check
 CHECK(kind IN ('RECEIPT','QUARANTINE_RELEASE_OUT','QUARANTINE_RELEASE_IN','DISPOSE','SALE','SALE_VOID','RETURN_QUARANTINE'));

-- Snapshot alokasi FEFO per baris penjualan (APT-005, APT-011): batch dan ED dibekukan saat finalisasi.
CREATE TABLE sale_line_batches (
 business_id UUID NOT NULL,
 sale_id UUID NOT NULL,
 sale_line_id UUID NOT NULL,
 batch_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 batch_number TEXT NOT NULL,
 expiry_date DATE NOT NULL,
 qty_milli BIGINT NOT NULL CHECK(qty_milli>0),
 returned_milli BIGINT NOT NULL DEFAULT 0 CHECK(returned_milli>=0 AND returned_milli<=qty_milli),
 override_reason TEXT NOT NULL DEFAULT '',
 PRIMARY KEY(business_id,sale_line_id,batch_id),
 FOREIGN KEY(business_id,sale_id) REFERENCES sales(business_id,id),
 FOREIGN KEY(business_id,batch_id) REFERENCES pharmacy_batches(business_id,id)
);
-- Referensi resep dan pemverifikasi apoteker (APT-008, APT-009); tidak dimasukkan ke struk publik.
ALTER TABLE sales ADD COLUMN prescription_reference TEXT, ADD COLUMN pharmacist_verified_by UUID REFERENCES users(id);
CREATE INDEX sale_line_batches_batch ON sale_line_batches(business_id,batch_id,sale_id);
CREATE INDEX sale_line_batches_sale ON sale_line_batches(business_id,sale_id);
COMMIT;
