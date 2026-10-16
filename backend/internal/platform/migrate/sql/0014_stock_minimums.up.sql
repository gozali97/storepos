BEGIN;
CREATE TABLE stock_minimums (
 business_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 variant_id UUID NOT NULL,
 minimum_qty_milli BIGINT NOT NULL CHECK(minimum_qty_milli>=0),
 version INT NOT NULL DEFAULT 1,
 updated_by UUID NOT NULL REFERENCES users(id),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(business_id,branch_id,variant_id),
 FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id),
 FOREIGN KEY(business_id,variant_id) REFERENCES variants(business_id,id)
);
CREATE INDEX goods_receipt_lines_variant ON goods_receipt_lines(business_id,variant_id,receipt_id);
COMMIT;
