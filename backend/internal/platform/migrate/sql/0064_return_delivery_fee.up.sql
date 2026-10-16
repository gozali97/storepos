BEGIN;
-- Snapshot ongkir yang direfund: nol untuk retur parsial, fee penjualan untuk retur penuh.
-- Retur historis dengan ongkir tidak direkonstruksi tanpa bukti; NULL menandakan laporan belum lengkap.
ALTER TABLE sale_returns ADD COLUMN delivery_fee_cents BIGINT CHECK(delivery_fee_cents>=0);
UPDATE sale_returns r SET delivery_fee_cents=0 FROM sales s WHERE s.business_id=r.business_id AND s.id=r.sale_id AND s.delivery_fee_cents=0;
ALTER TABLE sale_returns ALTER COLUMN delivery_fee_cents SET DEFAULT 0;
COMMIT;
