BEGIN;
ALTER TABLE variants ADD COLUMN default_purchase_unit_id UUID, ADD COLUMN default_sale_unit_id UUID, ADD COLUMN unit_defaults_version INT NOT NULL DEFAULT 1;
ALTER TABLE variants ADD CONSTRAINT variants_default_purchase_unit_fk FOREIGN KEY(business_id,id,default_purchase_unit_id) REFERENCES purchase_units(business_id,variant_id,id);
ALTER TABLE variants ADD CONSTRAINT variants_default_sale_unit_fk FOREIGN KEY(business_id,id,default_sale_unit_id) REFERENCES purchase_units(business_id,variant_id,id);
COMMIT;
