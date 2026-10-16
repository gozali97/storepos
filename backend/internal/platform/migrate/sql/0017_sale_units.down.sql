BEGIN;
ALTER TABLE sale_lines DROP COLUMN unit_id,DROP COLUMN sale_unit,DROP COLUMN factor_milli,DROP COLUMN base_qty_milli;
ALTER TABLE variant_barcodes DROP COLUMN unit_id;
DROP INDEX purchase_units_variant_id;
ALTER TABLE purchase_units DROP COLUMN sale_price_cents;
COMMIT;
