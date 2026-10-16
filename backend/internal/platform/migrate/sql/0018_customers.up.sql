BEGIN;
CREATE TABLE customers (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL REFERENCES businesses(id),
 name TEXT NOT NULL CHECK(length(btrim(name))>0),
 phone TEXT NOT NULL DEFAULT '',
 note TEXT NOT NULL DEFAULT '',
 anonymized BOOLEAN NOT NULL DEFAULT false,
 version INT NOT NULL DEFAULT 1,
 created_by UUID NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(business_id,id),
 CHECK(NOT anonymized OR (name='Pelanggan dihapus' AND phone='' AND note=''))
);
ALTER TABLE sales ADD COLUMN customer_id UUID;
ALTER TABLE sales ADD CONSTRAINT sales_customer_fk FOREIGN KEY(business_id,customer_id) REFERENCES customers(business_id,id);
COMMIT;
