BEGIN;
ALTER TABLE sales ADD COLUMN return_status TEXT NOT NULL DEFAULT 'none' CHECK(return_status IN ('none','partial','full'));
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_kind_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_kind_check CHECK(kind IN ('RECEIPT','SALE','SALE_VOID','RETURN_RESTOCK','RETURN_DAMAGED'));
CREATE UNIQUE INDEX sale_lines_business_id_id ON sale_lines(business_id,id);
CREATE TABLE sale_returns (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 sale_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 reason TEXT NOT NULL CHECK(length(btrim(reason))>0),
 amount_cents BIGINT NOT NULL CHECK(amount_cents>=0),
 created_by UUID NOT NULL REFERENCES users(id),
 approved_by UUID REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(business_id,id),
 FOREIGN KEY(business_id,sale_id) REFERENCES sales(business_id,id),
 FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id)
);
CREATE TABLE sale_return_lines (
 business_id UUID NOT NULL,
 return_id UUID NOT NULL,
 sale_line_id UUID NOT NULL,
 qty_milli BIGINT NOT NULL CHECK(qty_milli>0),
 amount_cents BIGINT NOT NULL CHECK(amount_cents>=0),
 condition TEXT NOT NULL CHECK(condition IN ('restock','damaged')),
 PRIMARY KEY(return_id,sale_line_id),
 FOREIGN KEY(business_id,return_id) REFERENCES sale_returns(business_id,id),
 FOREIGN KEY(business_id,sale_line_id) REFERENCES sale_lines(business_id,id)
);
CREATE TABLE refunds (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 return_id UUID NOT NULL,
 method TEXT NOT NULL,
 amount_cents BIGINT NOT NULL CHECK(amount_cents>0),
 shift_id UUID,
 manual_refund_required BOOLEAN NOT NULL,
 FOREIGN KEY(business_id,return_id) REFERENCES sale_returns(business_id,id),
 FOREIGN KEY(business_id,shift_id) REFERENCES shifts(business_id,id),
 CHECK((method='cash' AND shift_id IS NOT NULL) OR (method<>'cash' AND shift_id IS NULL))
);
CREATE INDEX sale_returns_sale ON sale_returns(business_id,sale_id);
CREATE INDEX refunds_shift ON refunds(business_id,shift_id);
COMMIT;
