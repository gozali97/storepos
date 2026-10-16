BEGIN;
CREATE TABLE report_exports (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL REFERENCES businesses(id),
 branch_id UUID NOT NULL,
 requested_by UUID NOT NULL REFERENCES users(id),
 report_kind TEXT NOT NULL CHECK(report_kind IN ('payments','sales','corrections','stock','products')),
 parameters JSONB NOT NULL,
 status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','completed','failed','expired')),
 cost_visible BOOLEAN NOT NULL,
 profit_visible BOOLEAN NOT NULL,
 row_count BIGINT,
 artifact BYTEA,
 error_code TEXT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 completed_at TIMESTAMPTZ,
 expires_at TIMESTAMPTZ NOT NULL DEFAULT now()+interval '7 days',
 FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id)
);
CREATE INDEX report_exports_pending ON report_exports(created_at,id) WHERE status='pending';
CREATE INDEX report_exports_expiry ON report_exports(expires_at) WHERE status<>'expired';
COMMIT;
