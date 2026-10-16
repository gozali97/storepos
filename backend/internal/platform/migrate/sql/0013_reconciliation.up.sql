BEGIN;
CREATE TABLE reconciliation_runs (
 run_date DATE PRIMARY KEY,
 started_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 completed_at TIMESTAMPTZ,
 finding_count BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE reconciliation_findings (
 id UUID PRIMARY KEY,
 run_date DATE NOT NULL REFERENCES reconciliation_runs(run_date),
 business_id UUID NOT NULL REFERENCES businesses(id),
 kind TEXT NOT NULL CHECK(kind IN ('stock','payment')),
 entity_key TEXT NOT NULL,
 expected_value NUMERIC NOT NULL,
 actual_value NUMERIC NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX reconciliation_findings_tenant ON reconciliation_findings(business_id,run_date,kind);
COMMIT;
