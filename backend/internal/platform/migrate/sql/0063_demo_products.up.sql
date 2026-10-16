BEGIN;
-- Penanda data contoh onboarding (§8.1): dibuat tanpa pelacakan stok sehingga tidak menyentuh kartu stok append-only,
-- dan "dihapus" dengan mengarsipkan agar riwayat transaksi uji tetap utuh.
ALTER TABLE products ADD COLUMN is_demo BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX products_demo ON products(business_id) WHERE is_demo;
COMMIT;
