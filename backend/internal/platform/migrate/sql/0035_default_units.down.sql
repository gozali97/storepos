BEGIN;
ALTER TABLE variants DROP COLUMN default_purchase_unit_id,DROP COLUMN default_sale_unit_id,DROP COLUMN unit_defaults_version;
COMMIT;
