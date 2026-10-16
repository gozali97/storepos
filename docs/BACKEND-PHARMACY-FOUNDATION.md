# Batch 90 — 20 bagian fondasi apotek

Modul pharmacy wajib. Semua endpoint sesi/tenant/izin/cabang, CSRF untuk mutasi.

- PUT /pharmacy/products/{product_id}, If-Match versi PRODUCT: active_ingredient,dosage_form,strength,manufacturer,registration_number,classification,prescription_required,special_restrictions. Data teks <=500 karakter, sistem tidak menentukan golongan. GET detail dan GET /pharmacy/products tersedia; memakai produk/katalog yang sudah dibuat. Drug detail drug_version berbeda dari product_version; baca product detail sebelum edit.
- POST /pharmacy/batch-receipts, Idempotency-Key: branch_id,variant_id,batch_number,expiry_date YYYY-MM-DD atau YYYY-MM,supplier_id,invoice_number,qty_milli,unit_cost_cents,status SELLABLE|QUARANTINE. Qty dalam satuan dasar dan integer unit (kelipatan 1000). Supplier aktif wajib, varian track_stock aktif nonpecahan. ED <=tanggal lokal cabang hanya QUARANTINE. Dokumen langsung posted, belum draft/conversion purchase-unit.
- GET /pharmacy/batch-receipts?branch_id&after: histori harga beli, izin batch.manage; kartu GET /pharmacy/batches/{id}/movements?branch_id&after.
- GET /pharmacy/batches?branch_id&filter=all|near_expiry|expired|quarantine&days=90: stok per status, expiry derived dan supplier terakhir. Maks 101 baris, truncated eksplisit, bukan cursor lengkap antar bucket; after batch ID dapat digunakan manual tetapi jangan melewati status lain pada batch yang sama.
- GET /pharmacy/fefo-preview?branch_id&variant_id&qty_milli: sellable ED>hari ini berurutan ED/id, membagi qty dan shortage. preview_only/finalization_blocked=true. Tidak ada lock/komit alokasi, kasir harus menghitung ulang saat final nanti.
- POST /pharmacy/batches/{id}/release: branch_id,qty_milli,reason; hanya Pharmacist, idempotency wajib. Karantina→sellable hanya ED valid, dua pergerakan dalam transaksi.
- POST /pharmacy/batches/{id}/dispose: branch_id,qty_milli,status sumber SELLABLE|QUARANTINE,reason; batch.manage, idempotency/audit. Status DISPOSED marker saldo 0 disimpan, bukan mengubah semua stok batch menjadi disposed bila masih tersisa.

Safety: finalisasi core untuk pharmacy diblokir. Tidak mengintegrasikan batch saldo ke inventory_balances/HPP moving-average core, sehingga core report stok belum merepresentasikan apotek. Resep/otorisasi apoteker/manual override/FEFO final/retur karantina/opname batch belum. Waste/dispose memerlukan verifikasi regulasi dan workflow approval lebih lanjut sebelum pilot. Klasifikasi masih teks, bukan master golongan tervalidasi.

Tidak menambah fungsi tes baru; build/vet/suite lama -race lulus, bukan bukti AC apotek lulus. Migrasi 0056. Frontend tidak diubah. Perlu manual/integration check batch/ED/stock/date/concurrency sebelum aktivasi pilot; bukan klaim kepatuhan.
