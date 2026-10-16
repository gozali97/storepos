BEGIN;
ALTER TABLE purchase_units ADD COLUMN sale_price_cents BIGINT CHECK(sale_price_cents>=0);
CREATE UNIQUE INDEX purchase_units_variant_id ON purchase_units(business_id,variant_id,id);
ALTER TABLE variant_barcodes ADD COLUMN unit_id UUID;
ALTER TABLE variant_barcodes ADD CONSTRAINT barcode_unit_fk FOREIGN KEY(business_id,variant_id,unit_id) REFERENCES purchase_units(business_id,variant_id,id);
ALTER TABLE sale_lines ADD COLUMN unit_id UUID, ADD COLUMN sale_unit TEXT, ADD COLUMN factor_milli BIGINT NOT NULL DEFAULT 1000 CHECK(factor_milli>0), ADD COLUMN base_qty_milli BIGINT;
UPDATE sale_lines SET sale_unit=base_unit,base_qty_milli=qty_milli;
ALTER TABLE sale_lines ALTER COLUMN base_qty_milli SET NOT NULL;
COMMIT;
