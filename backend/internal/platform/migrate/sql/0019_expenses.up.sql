BEGIN;
CREATE TABLE expenses (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 expense_date DATE NOT NULL,
 category TEXT NOT NULL CHECK(length(btrim(category))>0),
 amount_cents BIGINT NOT NULL CHECK(amount_cents>0),
 method TEXT NOT NULL CHECK(method IN ('shift_cash','outside_cash')),
 note TEXT NOT NULL DEFAULT '',
 shift_id UUID,
 created_by UUID NOT NULL REFERENCES users(id),
 approved_by UUID REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id),
 FOREIGN KEY(business_id,shift_id) REFERENCES shifts(business_id,id),
 CHECK((method='shift_cash' AND shift_id IS NOT NULL) OR (method='outside_cash' AND shift_id IS NULL))
);
CREATE INDEX expenses_report ON expenses(business_id,branch_id,expense_date,id);
COMMIT;
