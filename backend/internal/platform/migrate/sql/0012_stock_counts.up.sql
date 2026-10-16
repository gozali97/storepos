BEGIN;
CREATE TABLE stock_counts (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','posted')),
 version INT NOT NULL DEFAULT 1,
 created_by UUID NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 posted_by UUID REFERENCES users(id),
 posted_at TIMESTAMPTZ,
 UNIQUE(business_id,id),
 FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id)
);
CREATE TABLE stock_count_lines (
 business_id UUID NOT NULL,
 count_id UUID NOT NULL,
 variant_id UUID NOT NULL,
 snapshot_qty_milli BIGINT NOT NULL,
 system_qty_at_count BIGINT,
 counted_qty_milli BIGINT CHECK(counted_qty_milli>=0),
 counted_by UUID REFERENCES users(id),
 counted_at TIMESTAMPTZ,
 PRIMARY KEY(count_id,variant_id),
 FOREIGN KEY(business_id,count_id) REFERENCES stock_counts(business_id,id),
 FOREIGN KEY(business_id,variant_id) REFERENCES variants(business_id,id)
);
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_kind_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_kind_check CHECK(kind IN ('RECEIPT','SALE','SALE_VOID','RETURN_RESTOCK','RETURN_DAMAGED','ADJUSTMENT_IN','ADJUSTMENT_OUT','OPNAME_ADJUSTMENT'));
COMMIT;
