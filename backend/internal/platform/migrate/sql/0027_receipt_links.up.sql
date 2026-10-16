BEGIN;
CREATE TABLE receipt_links (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 sale_id UUID NOT NULL,
 token_hash TEXT NOT NULL UNIQUE,
 created_by UUID NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 revoked_at TIMESTAMPTZ,
 FOREIGN KEY(business_id,sale_id) REFERENCES sales(business_id,id)
);
CREATE INDEX receipt_links_sale ON receipt_links(business_id,sale_id);
COMMIT;
