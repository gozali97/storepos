# Batch 80 — impor katalog CSV

GET /catalog/import/template (product.manage) menghasilkan header:
product_name,variant_name,sku,base_unit,price_cents,barcode,allow_fractional,track_stock,opening_qty_milli,opening_cost_cents

POST /catalog/import/preview?branch_id=UUID dan /commit?branch_id=UUID menerima raw text/csv (bukan multipart). Maksimal 5 MiB/5.000 baris, UTF-8/BOM, header/urutan tepat. Semua nominal string integer sen pada CSV, qty integer milli. Boolean memakai format strconv.ParseBool (true/false disarankan). Barcode opsional, SKU wajib case-insensitive unik. HPP boleh kosong hanya jika opening qty nol; stok positif wajib HPP, termasuk nol bila memang biaya nol.

Preview menghasilkan row_count/valid/errors row+field. Commit wajib Idempotency-Key; hash branch+raw bytes. Preview dan commit memakai validasi sama, commit mengecek konflik DB lagi. Satu transaksi seluruh file; konflik satu baris rollback semua. Stok OPENING_BALANCE dan HPP ditulis bersama produk baru, tanpa menimpa saldo/master existing. Audit hanya jumlah baris/cabang, tidak file mentah.

Retail aktif saja, product.manage + cabang. Satu CSV row membuat satu product dan satu variant (tidak menggabungkan product_name yang sama); kategori, modifiers, multivariants per product, satuan tambahan dan apotek batch/ED belum didukung. Angka dibatasi harga/biaya Rp100 juta dan qty milli <=1e12 untuk mencegah input ekstrem. Error parse/header/structural belum semuanya berupa daftar per-row, beberapa HTTP 422 message.

Tanpa fungsi tes baru; suite lama -race lulus sebelum pembatasan numerik akhir; build/vet diulang sesudahnya. Endpoint impor belum diuji HTTP/rollback khusus, ukuran 5.000 row/DB konflik dicek satu query per baris sehingga belum benchmark 5.000. Frontend tidak diubah.

Rekap terbaru: checkbox MVP 93 terbuka (19 fondasi/45 retail/11 restoran/18 apotek), campuran backend/UI/QA/infra, bukan jumlah backend eksak. Fokus fitur berikutnya sebaiknya onboarding/registrasi, refund split dan alur apotek, bukan mengurangi hitungan dengan endpoint detail tambahan.
