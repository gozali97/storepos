BEGIN;
CREATE TABLE sale_drafts (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL REFERENCES businesses(id),
 branch_id UUID NOT NULL,
 register_id UUID NOT NULL,
 status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','held','cancelled','completed')),
 version INT NOT NULL DEFAULT 1,
 payload JSONB NOT NULL,
 completed_sale_id UUID,
 created_by UUID NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id),
 FOREIGN KEY(business_id,register_id) REFERENCES registers(business_id,id),
 FOREIGN KEY(business_id,completed_sale_id) REFERENCES sales(business_id,id)
);
CREATE UNIQUE INDEX sale_drafts_one_active ON sale_drafts(business_id,register_id) WHERE status='active';
CREATE INDEX sale_drafts_terminal ON sale_drafts(business_id,register_id,status);
COMMIT;
