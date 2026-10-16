BEGIN;
ALTER TABLE sale_lines DROP COLUMN tax_rule_name,DROP COLUMN tax_rate_bps,DROP COLUMN tax_mode;
ALTER TABLE products DROP COLUMN tax_rule_id;
ALTER TABLE categories DROP COLUMN tax_rule_id;
DROP TABLE scoped_tax_rules;
COMMIT;
