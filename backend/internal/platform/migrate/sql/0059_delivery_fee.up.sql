BEGIN;
-- Ongkir restoran (DELIVERY_MANUAL): biaya terpisah yang ikut total tagihan,
-- tidak kena pajak, diskon, maupun service charge.
ALTER TABLE sales ADD COLUMN delivery_fee_cents BIGINT NOT NULL DEFAULT 0 CHECK (delivery_fee_cents >= 0);
COMMIT;
