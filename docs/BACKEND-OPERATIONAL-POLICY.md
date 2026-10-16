# Kebijakan operasional

GET /api/v1/settings/operational-policy: `{policy,version}`. PUT endpoint sama dengan If-Match dan payload lengkap:

```json
{"cashier_discount_bps":1000,"cash_out_approval_cents":"50000000","variance_approval_cents":"1000000","cash_rounding_enabled":true,"cash_rounding_multiple_cents":"10000","return_window_days":0}
```

Batas diskon 1000 bps = 10%; uang integer string sen. Default kas keluar Rp500.000, selisih kas Rp10.000. Pemilik saja (business.manage). Nilai diskon 0..10000, ambang nonnegatif maksimal Rp1 miliar. PUT mengganti seluruh kebijakan; GET dahulu agar tidak mengosongkan nilai. Versi stale ditolak dan perubahan diaudit. Settings bisnis lainnya dipertahankan memakai jsonb_set.

API finalisasi memakai batas diskon saat request diproses; Pemilik/Manajer tetap punya override sesuai matriks. CashMove dan CloseShift membaca ambang di transaksi bisnis; pengeluaran shift juga mengikuti CashMove. Aturan persetujuan adalah nominal > ambang, bukan >=. Default dipakai jika policy belum tersimpan.

Toggle pembulatan/kelipatan sekarang tersedia: default aktif Rp100, kelipatan Rp1..Rp1.000 tanpa pecahan rupiah; hanya semua pembayaran tunai. Settings lama tanpa field baru memakai default agar kompatibel. Field baru yang dihilangkan dari PUT memakai default, bukan nilai sebelumnya; gunakan GET dan kirim payload lengkap. Pembulatan tersimpan pada snapshot penjualan; refund penuh memakai nilai transaksi asal.

Batas hari retur tersedia melalui return_window_days (0..3650), default 0 tanpa batas. Retur dibolehkan sampai akhir tanggal lokal penjualan + N hari (inklusif), tidak dihitung sebagai N×24 jam. Kebijakan berlaku saat request retur; penolakan tidak mengubah stok/refund. Hari ini dibaca dari jam PostgreSQL, zona waktu dari cabang asal.

Belum: batas per role kustom, stok negatif, pajak kategori/produk/cabang, blind-count toggle, dan UI. Perubahan kebijakan diskon bersamaan dengan request finalisasi dapat berlaku mulai request berikutnya; limit dibaca sebelum transaksi finalisasi. Frontend tidak diubah.
