# Penerimaan barang dan HPP

- `POST /api/v1/purchase-units`: `{variant_id, name, factor_milli}`. Dus berisi 24 pcs memakai faktor `"24000"`.
- `POST /api/v1/goods-receipts`: `{branch_id, invoice_number, received_date, lines:[{unit_id,qty_milli,purchase_price_cents}]}`. Kuantitas 2 Dus memakai `"2000"`; harga per Dus Rp48.000 memakai `"4800000"`. Dokumen draft tidak menambah stok.
- `POST /api/v1/goods-receipts/{id}/post`, `If-Match: 1`: posting atomik, saldo +48 pcs dan HPP dasar Rp2.000 pada contoh di atas. Posting ulang ditolak tanpa efek tambahan.

Pemilik/Manajer/Gudang dengan izin terkait dan penugasan cabang dapat menerima barang. Apotek diblokir pada jalur ini sampai penerimaan batch/ED tersedia.

HPP dihitung per varian/cabang dengan rata-rata tertimbang bergerak, dibulatkan half-up ke sen menggunakan integer presisi besar. Konversi yang kehilangan presisi milli-unit ditolak. Faktor konversi yang sudah dipakai tidak boleh diubah. Snapshot faktor, satuan, kuantitas dasar dan harga dasar tersimpan per baris penerimaan.

Saldo lama yang masih positif tetapi HPP belum diketahui memblokir posting; jangan menganggap biaya sebelumnya nol. Belum ada endpoint penetapan HPP awal. Jalur lama `/stock/receipts` masih tidak menghitung HPP dan tidak disarankan untuk pembelian berbiaya.

GET /goods-receipts/{id} kini membaca status/version/posted_at/header/supplier dan semua baris snapshot, dengan tenant/cabang dan lock share agar detail konsisten saat posting. Setelah timeout posting, baca endpoint ini: jika posted jangan posting ulang; jika draft, gunakan versi terkini.

PUT /goods-receipts/{id}, If-Match: {received_date,invoice_number,supplier_id,lines:[{unit_id,qty_milli,purchase_price_cents}]} mengganti seluruh header/baris draft, bukan patch parsial. Cabang tidak dapat dipindahkan. supplier_id null/absen menghapus supplier. Supplier harus aktif dan satuan tenant yang sama. Dokumen posted atau versi stale ditolak 409. Penggantian baris dan audit atomik; edit tidak menambah stok.

Supplier, HPP snapshot penjualan, satuan jual/barcode dan opname sudah tersedia pada endpoint terkait. GET /goods-receipts?branch_id=UUID&from=YYYY-MM-DD&to=YYYY-MM-DD&status=draft|posted&invoice_number=...&supplier_id=UUID&limit=100&before=UUID kini tersedia. Periode tanggal dokumen received_date wajib, maksimum 366 hari; faktur eksak. Paginasi created_at/id stabil pada timestamp sama, cursor wajib sesuai cakupan/filter. Supplier arsip tetap tampil sebagai histori. API no-store dengan izin goods.receive dan akses tenant/cabang. Migrasi 0034 menambah indeks pencarian.

Posting kini menerima Idempotency-Key bersama If-Match. Key sama + ID/versi sama mengembalikan hasil tersimpan (Idempotent-Replay:true), tanpa efek stok/HPP kedua. Key sama/versi berbeda ditolak IDEMPOTENCY_CONFLICT. Identitas hash mencakup ID dokumen dan versi; endpoint post tidak menerima payload bisnis. Pemeriksaan cabang dilakukan sebelum replay. Key sementara opsional untuk kompatibilitas klien lama; tanpa key tetap proteksi versi/post-once, gunakan GET status setelah timeout.

Belum: kewajiban key setelah migrasi klien frontend, penerimaan apotek batch/ED. HPP awal kini dapat diisi melalui PUT /stock/variants/{id}/opening-cost oleh Pemilik jika saldo positif dan biaya masih NULL; lihat BACKEND-OPENING-COST.md. Agent frontend perlu menyambungkan edit/detail/status; file frontend tidak diubah.
