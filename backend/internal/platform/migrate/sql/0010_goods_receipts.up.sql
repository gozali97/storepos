BEGIN;
CREATE TABLE purchase_units (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 variant_id UUID NOT NULL,
 name TEXT NOT NULL CHECK(length(btrim(name))>0),
 factor_milli BIGINT NOT NULL CHECK(factor_milli>0),
 used_at TIMESTAMPTZ,
 archived BOOLEAN NOT NULL DEFAULT false,
 UNIQUE(business_id,id),
 FOREIGN KEY(business_id,variant_id) REFERENCES variants(business_id,id)
);
CREATE UNIQUE INDEX purchase_units_name ON purchase_units(business_id,variant_id,lower(name)) WHERE NOT archived;
CREATE FUNCTION purchase_units_lock_factor() RETURNS trigger AS $$
BEGIN
 IF OLD.used_at IS NOT NULL AND (NEW.factor_milli<>OLD.factor_milli OR NEW.variant_id<>OLD.variant_id OR NEW.business_id<>OLD.business_id) THEN
 RAISE EXCEPTION 'satuan yang sudah dipakai tidak boleh diubah faktor atau variannya';
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER purchase_units_lock BEFORE UPDATE ON purchase_units FOR EACH ROW EXECUTE FUNCTION purchase_units_lock_factor();
CREATE TABLE goods_receipts (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 invoice_number TEXT NOT NULL DEFAULT '',
 received_date DATE NOT NULL,
 status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','posted')),
 version INT NOT NULL DEFAULT 1,
 created_by UUID NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 posted_at TIMESTAMPTZ,
 UNIQUE(business_id,id),
 FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id)
);
CREATE TABLE goods_receipt_lines (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 receipt_id UUID NOT NULL,
 unit_id UUID NOT NULL,
 qty_milli BIGINT NOT NULL CHECK(qty_milli>0),
 purchase_price_cents BIGINT NOT NULL CHECK(purchase_price_cents>=0),
 variant_id UUID,
 unit_name TEXT,
 factor_milli BIGINT,
 base_qty_milli BIGINT,
 base_cost_cents BIGINT,
 FOREIGN KEY(business_id,receipt_id) REFERENCES goods_receipts(business_id,id),
 FOREIGN KEY(business_id,unit_id) REFERENCES purchase_units(business_id,id),
 FOREIGN KEY(business_id,variant_id) REFERENCES variants(business_id,id)
);
ALTER TABLE inventory_balances ADD COLUMN average_cost_cents BIGINT CHECK(average_cost_cents>=0);
COMMIT;
