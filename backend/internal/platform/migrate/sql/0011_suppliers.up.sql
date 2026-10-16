BEGIN;
CREATE TABLE suppliers (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL REFERENCES businesses(id),
 name TEXT NOT NULL CHECK(length(btrim(name))>0),
 contact TEXT NOT NULL DEFAULT '',
 address TEXT NOT NULL DEFAULT '',
 note TEXT NOT NULL DEFAULT '',
 archived BOOLEAN NOT NULL DEFAULT false,
 version INT NOT NULL DEFAULT 1,
 created_by UUID NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(business_id,id)
);
CREATE INDEX suppliers_business_name ON suppliers(business_id,lower(name),id);
ALTER TABLE goods_receipts ADD COLUMN supplier_id UUID;
ALTER TABLE goods_receipts ADD CONSTRAINT goods_receipts_supplier_fk FOREIGN KEY(business_id,supplier_id) REFERENCES suppliers(business_id,id);
COMMIT;
