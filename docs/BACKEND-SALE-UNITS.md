# Satuan jual tambahan — RTL-002

Tabel purchase_units yang sudah digunakan penerimaan kini dapat memiliki harga jual tersendiri.

- Buat satuan: POST /purchase-units dengan variant_id/name/factor_milli.
- PUT /purchase-units/{id}/sale-price: `{sale_price_cents:"4800000"}`; tidak otomatis diturunkan dari harga dasar.
- POST /purchase-units/{id}/barcodes: `{barcode:"899-DUS"}`; barcode tetap unik per bisnis, FK mengikat unit pada varian yang benar.
- GET /catalog/barcode?barcode=...: resolusi varian, unit, faktor dan harga jual. Barcode dasar mengembalikan unit_id null.
- GET /variants/{id}/units: satuan tambahan aktif, harga nullable, serta penanda factor_locked.

Baris draft/finalisasi dapat menyertakan unit_id. qty_milli memakai satuan yang dijual; 2 Dus = 2000. Tanpa unit_id, harga/kuantitas tetap satuan dasar untuk kompatibilitas. Finalisasi membaca harga/faktor server, mengurangi stok dalam satuan dasar dan mengunci faktor yang sudah dipakai.

Sale_lines menyimpan unit, faktor, harga, qty jual dan qty dasar. Retur menerima qty dalam satuan jual asli, mengembalikan stok dengan faktor snapshot. Contoh teruji: 2 Dus ×24 pcs @Rp48.000 menghasilkan total Rp96.000, stok -48 pcs; retur 1 Dus refund Rp48.000 dan stok +24 pcs walaupun harga master berubah.

GET/PUT /variants/{id}/unit-defaults tersedia: default_purchase_unit_id/default_sale_unit_id nullable (null = satuan dasar), version terpisah dan If-Match. Unit harus varian/tenant sama dan nonarsip; default jual harus memiliki harga. FK DB komposit menjaga relasi dan perubahan teraudit. Default adalah preferensi pilihan klien, bukan penggantian otomatis: finalisasi tanpa unit_id tetap satuan dasar. Frontend tidak diubah; pemilih default perlu membaca endpoint ini.

PUT /purchase-units/{id}, If-Match wajib, {name,factor_milli,archived} kini tersedia. Faktor unit used_at terkunci; arsip unit yang masih default ditolak, ubah default terlebih dahulu. Histori snapshot tetap ada, unit tidak dihapus. Daftar unit mengembalikan version. Harga /sale-price menerima If-Match opsional dan menaikkan version; header stale ditolak. Tanpa header masih last-write-wins untuk kompatibilitas, perlu migrasi klien frontend sebelum diwajibkan.

Belum: penyambungan UI edit/arsip/default dan kewajiban versi harga. Tes fitur baru tidak ditambah; suite lama/build/vet dijalankan. Finalisasi multi-unit dapat memerlukan retry deadlock bila urutan unit berbeda; pool menyediakan retry transaksi.
