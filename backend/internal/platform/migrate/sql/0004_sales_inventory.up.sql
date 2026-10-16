-- 0004 — Stok, shift, penjualan, pembayaran (CORE-017, CORE-024, CORE-032, CORE-046)
BEGIN;

CREATE UNIQUE INDEX registers_business_id_id_key ON registers(business_id, id);

CREATE TABLE inventory_balances (
    business_id UUID NOT NULL,
    branch_id   UUID NOT NULL,
    variant_id  UUID NOT NULL,
    qty_milli   BIGINT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (business_id, branch_id, variant_id),
    FOREIGN KEY (business_id, branch_id)  REFERENCES branches(business_id, id),
    FOREIGN KEY (business_id, variant_id) REFERENCES variants(business_id, id)
);

-- Pergerakan stok append-only; qty bertanda (+ masuk, - keluar); balance_after untuk kartu stok.
CREATE TABLE stock_movements (
    id                  UUID PRIMARY KEY,
    business_id         UUID NOT NULL,
    branch_id           UUID NOT NULL,
    variant_id          UUID NOT NULL,
    kind                TEXT NOT NULL CHECK (kind IN ('RECEIPT', 'SALE')),
    qty_milli           BIGINT NOT NULL CHECK (qty_milli <> 0),
    balance_after_milli BIGINT NOT NULL,
    ref_type            TEXT NOT NULL,
    ref_id              UUID NOT NULL,
    created_by          UUID NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (business_id, branch_id)  REFERENCES branches(business_id, id),
    FOREIGN KEY (business_id, variant_id) REFERENCES variants(business_id, id)
);
CREATE INDEX stock_movements_card_idx ON stock_movements(business_id, branch_id, variant_id, created_at);

CREATE OR REPLACE FUNCTION stock_movements_block_mutation() RETURNS trigger AS $$
BEGIN RAISE EXCEPTION 'stock_movements bersifat append-only'; END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER stock_movements_no_update BEFORE UPDATE ON stock_movements
    FOR EACH ROW EXECUTE FUNCTION stock_movements_block_mutation();
CREATE TRIGGER stock_movements_no_delete BEFORE DELETE ON stock_movements
    FOR EACH ROW EXECUTE FUNCTION stock_movements_block_mutation();

CREATE TABLE shifts (
    id                 UUID PRIMARY KEY,
    business_id        UUID NOT NULL,
    branch_id          UUID NOT NULL,
    register_id        UUID NOT NULL,
    opened_by          UUID NOT NULL REFERENCES users(id),
    opening_cash_cents BIGINT NOT NULL CHECK (opening_cash_cents >= 0),
    status             TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    opened_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at          TIMESTAMPTZ,
    UNIQUE (business_id, id),
    FOREIGN KEY (business_id, branch_id)   REFERENCES branches(business_id, id),
    FOREIGN KEY (business_id, register_id) REFERENCES registers(business_id, id)
);
-- Satu shift terbuka per terminal (CORE-024).
CREATE UNIQUE INDEX shifts_one_open_per_register ON shifts(register_id) WHERE status = 'open';

CREATE TABLE sales (
    id                        UUID PRIMARY KEY,
    business_id               UUID NOT NULL,
    branch_id                 UUID NOT NULL,
    register_id               UUID NOT NULL,
    shift_id                  UUID NOT NULL,
    receipt_number            TEXT NOT NULL,
    status                    TEXT NOT NULL DEFAULT 'completed' CHECK (status IN ('completed')),
    subtotal_gross_cents      BIGINT NOT NULL,
    line_discount_cents       BIGINT NOT NULL,
    tx_discount_cents         BIGINT NOT NULL,
    service_charge_cents      BIGINT NOT NULL,
    tax_cents                 BIGINT NOT NULL,
    cash_rounding_cents       BIGINT NOT NULL,
    total_cents               BIGINT NOT NULL CHECK (total_cents > 0),
    change_cents              BIGINT NOT NULL CHECK (change_cents >= 0),
    created_by                UUID NOT NULL REFERENCES users(id),
    completed_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (business_id, id),
    UNIQUE (business_id, branch_id, receipt_number),
    FOREIGN KEY (business_id, branch_id)   REFERENCES branches(business_id, id),
    FOREIGN KEY (business_id, register_id) REFERENCES registers(business_id, id),
    FOREIGN KEY (business_id, shift_id)    REFERENCES shifts(business_id, id)
);

-- Snapshot per baris: perubahan master data tidak mengubah transaksi lampau (§13.6).
CREATE TABLE sale_lines (
    id                  UUID PRIMARY KEY,
    business_id         UUID NOT NULL,
    sale_id             UUID NOT NULL,
    line_no             INT NOT NULL,
    variant_id          UUID NOT NULL,
    sku                 TEXT NOT NULL,
    product_name        TEXT NOT NULL,
    variant_name        TEXT NOT NULL,
    base_unit           TEXT NOT NULL,
    qty_milli           BIGINT NOT NULL CHECK (qty_milli > 0),
    unit_price_cents    BIGINT NOT NULL,
    gross_cents         BIGINT NOT NULL,
    line_discount_cents BIGINT NOT NULL,
    alloc_discount_cents BIGINT NOT NULL,
    net_cents           BIGINT NOT NULL,
    service_charge_cents BIGINT NOT NULL,
    tax_cents           BIGINT NOT NULL,
    line_total_cents    BIGINT NOT NULL,
    UNIQUE (sale_id, line_no),
    FOREIGN KEY (business_id, sale_id)    REFERENCES sales(business_id, id),
    FOREIGN KEY (business_id, variant_id) REFERENCES variants(business_id, id)
);

CREATE TABLE payments (
    id             UUID PRIMARY KEY,
    business_id    UUID NOT NULL,
    sale_id        UUID NOT NULL,
    method         TEXT NOT NULL CHECK (method IN ('cash', 'qris', 'card', 'transfer')),
    amount_cents   BIGINT NOT NULL CHECK (amount_cents > 0),
    tendered_cents BIGINT NOT NULL,
    change_cents   BIGINT NOT NULL DEFAULT 0,
    verification   TEXT NOT NULL DEFAULT 'MANUAL',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (business_id, sale_id) REFERENCES sales(business_id, id)
);

COMMIT;
