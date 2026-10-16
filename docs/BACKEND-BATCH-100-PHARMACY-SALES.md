# Batch 100 — penjualan apotek berbasis batch

Finalisasi `POST /sales/finalize` pada bisnis apotek: tiap baris dialokasikan FEFO di server dalam transaksi (baris saldo batch dikunci urut varian, ED, ID). Snapshot batch/ED/qty disimpan di `sale_line_batches`. Body tambahan: `prescription_reference`, `pharmacist_verification {email,pin}`, dan per baris `batch_id`+`batch_reason` (override, hanya apoteker). Produk `prescription_required` menolak tanpa referensi + verifikasi apoteker (422 PRESCRIPTION_REFERENCE_REQUIRED). Batch ED <= tanggal lokal cabang tidak dapat dijual (BATCH_EXPIRED).

Void mengembalikan sisa alokasi ke batch SELLABLE. Retur restock masuk QUARANTINE batch asal (dilepas apoteker lewat /release); retur rusak hanya menutup alokasi. GET /pharmacy/batches/{id}/trace menampilkan penerimaan dan penjualan tanpa data pelanggan. POST /pharmacy/batches/{id}/count menyamakan saldo batch/status dengan hitung fisik.

Batasan: saldo batch terpisah dari inventory_balances inti, sehingga laporan stok/HPP inti belum mencakup apotek. Refund split tender masih diblokir. Tanpa tes baru; wajib diuji end-to-end/konkurensi sebelum pilot. Validasi regulasi apoteker tetap terbuka.
