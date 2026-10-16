BEGIN;
CREATE TABLE pharmacy_drugs (
 business_id UUID NOT NULL,product_id UUID NOT NULL,
 active_ingredient TEXT NOT NULL DEFAULT '',dosage_form TEXT NOT NULL DEFAULT '',strength TEXT NOT NULL DEFAULT '',manufacturer TEXT NOT NULL DEFAULT '',registration_number TEXT NOT NULL DEFAULT '',classification TEXT NOT NULL DEFAULT '',prescription_required BOOLEAN NOT NULL DEFAULT false,special_restrictions BOOLEAN NOT NULL DEFAULT false,version INT NOT NULL DEFAULT 1,
 PRIMARY KEY(business_id,product_id),FOREIGN KEY(business_id,product_id) REFERENCES products(business_id,id)
);
CREATE TABLE pharmacy_batches (
 id UUID PRIMARY KEY,business_id UUID NOT NULL,variant_id UUID NOT NULL,batch_number TEXT NOT NULL,expiry_date DATE NOT NULL,
 UNIQUE(business_id,id),UNIQUE(business_id,variant_id,batch_number,expiry_date),FOREIGN KEY(business_id,variant_id) REFERENCES variants(business_id,id)
);
CREATE TABLE pharmacy_batch_balances (
 business_id UUID NOT NULL,branch_id UUID NOT NULL,batch_id UUID NOT NULL,status TEXT NOT NULL CHECK(status IN ('SELLABLE','QUARANTINE','DISPOSED')),qty_milli BIGINT NOT NULL CHECK(qty_milli>=0),
 PRIMARY KEY(business_id,branch_id,batch_id,status),FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id),FOREIGN KEY(business_id,batch_id) REFERENCES pharmacy_batches(business_id,id),CHECK(status<>'DISPOSED' OR qty_milli=0)
);
CREATE TABLE pharmacy_batch_receipts (
 id UUID PRIMARY KEY,business_id UUID NOT NULL,branch_id UUID NOT NULL,batch_id UUID NOT NULL,supplier_id UUID NOT NULL,invoice_number TEXT NOT NULL,qty_milli BIGINT NOT NULL CHECK(qty_milli>0),unit_cost_cents BIGINT NOT NULL CHECK(unit_cost_cents>=0),status TEXT NOT NULL CHECK(status IN ('SELLABLE','QUARANTINE')),created_by UUID NOT NULL REFERENCES users(id),created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id),FOREIGN KEY(business_id,batch_id) REFERENCES pharmacy_batches(business_id,id),FOREIGN KEY(business_id,supplier_id) REFERENCES suppliers(business_id,id)
);
CREATE TABLE pharmacy_batch_movements (
 id UUID PRIMARY KEY,business_id UUID NOT NULL,branch_id UUID NOT NULL,batch_id UUID NOT NULL,status TEXT NOT NULL,kind TEXT NOT NULL CHECK(kind IN ('RECEIPT','QUARANTINE_RELEASE_OUT','QUARANTINE_RELEASE_IN','DISPOSE')),qty_milli BIGINT NOT NULL CHECK(qty_milli<>0),balance_after_milli BIGINT NOT NULL CHECK(balance_after_milli>=0),ref_id UUID NOT NULL,reason TEXT NOT NULL DEFAULT '',created_by UUID NOT NULL REFERENCES users(id),created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(business_id,branch_id,batch_id,status) REFERENCES pharmacy_batch_balances(business_id,branch_id,batch_id,status)
);
CREATE INDEX pharmacy_batch_fefo ON pharmacy_batches(business_id,variant_id,expiry_date,id);
CREATE INDEX pharmacy_batch_card ON pharmacy_batch_movements(business_id,branch_id,batch_id,created_at,id);
CREATE TRIGGER pharmacy_movements_no_update BEFORE UPDATE ON pharmacy_batch_movements FOR EACH ROW EXECUTE FUNCTION stock_movements_block_mutation();
CREATE TRIGGER pharmacy_movements_no_delete BEFORE DELETE ON pharmacy_batch_movements FOR EACH ROW EXECUTE FUNCTION stock_movements_block_mutation();
COMMIT;
