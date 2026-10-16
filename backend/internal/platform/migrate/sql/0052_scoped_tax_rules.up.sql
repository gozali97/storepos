BEGIN;
CREATE TABLE scoped_tax_rules (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL REFERENCES businesses(id),
 name TEXT NOT NULL CHECK(char_length(name) BETWEEN 1 AND 100),
 rate_bps INT NOT NULL CHECK(rate_bps BETWEEN 0 AND 10000),
 mode TEXT NOT NULL CHECK(mode IN ('exclusive','inclusive')),
 archived BOOLEAN NOT NULL DEFAULT false,
 version INT NOT NULL DEFAULT 1,
 UNIQUE(business_id,id)
);
ALTER TABLE products ADD COLUMN tax_rule_id UUID;
ALTER TABLE products ADD CONSTRAINT products_scoped_tax_fk FOREIGN KEY(business_id,tax_rule_id) REFERENCES scoped_tax_rules(business_id,id);
ALTER TABLE categories ADD COLUMN tax_rule_id UUID;
ALTER TABLE categories ADD CONSTRAINT categories_scoped_tax_fk FOREIGN KEY(business_id,tax_rule_id) REFERENCES scoped_tax_rules(business_id,id);
ALTER TABLE sale_lines ADD COLUMN tax_rule_name TEXT,ADD COLUMN tax_rate_bps INT,ADD COLUMN tax_mode TEXT;
UPDATE sale_lines l SET tax_rule_name=s.tax_rule_name,tax_rate_bps=s.tax_rate_bps,tax_mode=s.tax_mode FROM sales s WHERE s.business_id=l.business_id AND s.id=l.sale_id;
COMMIT;
