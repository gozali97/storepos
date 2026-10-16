-- 0005 — Kas masuk/keluar, tutup shift, aturan pajak, snapshot diskon/pajak
-- (CORE-016, CORE-025, CORE-026)
BEGIN;

CREATE TABLE cash_movements (
    id           UUID PRIMARY KEY,
    business_id  UUID NOT NULL,
    shift_id     UUID NOT NULL,
    kind         TEXT NOT NULL CHECK (kind IN ('in', 'out')),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    reason       TEXT NOT NULL CHECK (length(btrim(reason)) > 0),
    approved_by  UUID REFERENCES users(id),
    created_by   UUID NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (business_id, shift_id) REFERENCES shifts(business_id, id)
);
CREATE INDEX cash_movements_shift_idx ON cash_movements(business_id, shift_id);

ALTER TABLE shifts
    ADD COLUMN counted_cash_cents  BIGINT,
    ADD COLUMN expected_cash_cents BIGINT,
    ADD COLUMN variance_cents      BIGINT,
    ADD COLUMN close_note          TEXT,
    ADD COLUMN closed_by           UUID REFERENCES users(id),
    ADD COLUMN close_approved_by   UUID REFERENCES users(id);

-- Shift yang sudah ditutup terkunci (CORE-026).
CREATE OR REPLACE FUNCTION shifts_lock_closed() RETURNS trigger AS $$
BEGIN
    IF OLD.status = 'closed' THEN
        RAISE EXCEPTION 'shift yang sudah ditutup tidak dapat diubah';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER shifts_lock_closed BEFORE UPDATE ON shifts
    FOR EACH ROW EXECUTE FUNCTION shifts_lock_closed();

-- Satu aturan pajak aktif per bisnis. Sistem tidak menentukan tarif (CORE-016 [VAL]).
CREATE TABLE tax_rules (
    id          UUID PRIMARY KEY,
    business_id UUID NOT NULL REFERENCES businesses(id),
    name        TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    rate_bps    INT  NOT NULL CHECK (rate_bps >= 0 AND rate_bps <= 10000),
    mode        TEXT NOT NULL CHECK (mode IN ('exclusive', 'inclusive')),
    active      BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX tax_rules_one_active ON tax_rules(business_id) WHERE active;

-- Snapshot: perubahan tarif tidak mengubah transaksi lampau.
ALTER TABLE sales
    ADD COLUMN tax_rule_name        TEXT,
    ADD COLUMN tax_rate_bps         INT,
    ADD COLUMN tax_mode             TEXT,
    ADD COLUMN discount_reason      TEXT,
    ADD COLUMN discount_approved_by UUID REFERENCES users(id);

COMMIT;
