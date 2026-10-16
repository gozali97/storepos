# Batch 70 — katalog

- GET /categories/{id}; PUT endpoint sama If-Match {name}; POST /categories/{id}/restore If-Match (tanpa body). Arsip lewat DELETE yang sudah tersedia. Restore menolak nama aktif duplikat.
- GET /products/{id}; PUT If-Match {name,category_id:UUID|null}; PATCH /products/{id}/status If-Match {status:active|inactive|archived}. Track_stock tidak diedit. Kategori harus aktif dan tenant sama. Status nonaktif ditolak bila ada item order open, tetapi pemeriksaan belum diserialkan dengan add item bersamaan; checkout masih dapat menolak varian/master nonaktif.
- PATCH /variants/{id}/identity If-Match {name,sku}, SKU unik case-insensitive. Tidak mengubah SKU snapshot lama.
- PATCH /variants/{id}/fractional If-Match {allow_fractional:boolean}. Pharmacy true ditolak. Snapshot histori tetap, perubahan kebijakan dapat menghalangi edit/checkout order lama; penyempurnaan snapshot policy order belum.
- GET /variants/{id}/barcodes?after=barcode: barcode/unit, 100/cursor.
- DELETE /variants/{id}/barcodes If-Match {barcode}: penghapusan mapping dan bump versi/audit atomik. Barcode snapshot transaksi historis tidak direkonstruksi/dihapus.

Read product.view, mutations product.manage, modul katalog core tidak dibatasi restaurant/pharmacy secara umum. Semua tenant dan CSRF. Metadata audit bukan customer PII. Tidak ada frontend/migrasi baru/fungsi tes baru; build/vet/suite lama -race lulus. Pengujian khusus endpoint/konkurensi batch belum.

Sisa hitungan task MVP masih 95 checkbox (19 fondasi/47 retail/11 restoran/18 apotek). Batch fitur rinci tidak identik dengan kebutuhan PRD induk. Estimasi backend substantif 35–45 kelompok, belum normalized exact ledger. Fitur utama yang belum: registrasi/verifikasi, impor CSV/opening balance, refund split, metode bayar kustom, checkout ongkir/restoran edge cases, seluruh alur batch/FEFO/resep apotek, MFA/support platform, hardening RLS/role deploy, validation/performance tersisa.
