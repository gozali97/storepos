# Rekonsiliasi — CORE-032 / NFR-012

Jalankan `storepos worker` (konfigurasi backend/.env). Saat startup dan setiap jam, worker mencoba claim tanggal UTC hari ini. Satu claim per tanggal; transaksi claim/temuan/status berjalan atomik. Crash membatalkan claim supaya dapat dicoba lagi. Beberapa worker tidak menggandakan hasil.

Invariant dalam satu snapshot SQL:

1. Saldo stok sama dengan jumlah pergerakan yang mengubah saldo jual. RETURN_DAMAGED dikecualikan karena hanya mencatat barang rusak, tidak menambah stok jual.
2. Jumlah amount_cents pembayaran confirmed sama dengan total penjualan completed; voided harus tidak memiliki pembayaran confirmed. Refund adalah dokumen terpisah, bukan pengurangan pembayaran asli.

Hasil disimpan di reconciliation_runs/reconciliation_findings. Worker mengeluarkan error log `reconciliation discrepancy` berisi jumlah temuan; tidak ada perbaikan saldo otomatis.

`GET /api/v1/reconciliation/findings?date=YYYY-MM-DD` membaca temuan tenant aktif, memerlukan audit.view. Maksimal 1000 temuan, field truncated=true menandai daftar tidak lengkap. Angka berupa string. Jadwal tanggal UTC, bukan tanggal bisnis cabang. Belum ada email/pager/webhook; operator harus memonitor error log.

Cakupan belum mencakup alokasi refund per metode, HPP/penilaian stok, saldo batch apotek, atau pemeriksaan orphan payment jika constraint dimatikan. Uji performa produksi dan konfigurasi monitoring eksternal belum dilakukan.
