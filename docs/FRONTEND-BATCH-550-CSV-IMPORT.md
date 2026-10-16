# B550 — UI impor katalog CSV

Migrasi lokal sampai 0065 applied; runner tidak menemukan migrasi baru.

- [x] Halaman /impor-katalog, link katalog/onboarding, guard retail owner/manager/cabang.
- [x] Template server, file UTF-8 <=5MiB, preview validasi per row/field, konfirmasi commit atomik.
- [x] API client rawBody CSV dan pending snapshot rawBody asli, retry idempotent, block aksi baru saat pending.
- [x] DITUNDA: testing/vet/build aplikasi/browser BELUM DIJALANKAN/BELUM LULUS.

Batas: storage quota browser mungkin menolak file besar sebelum request. Recovery hanya tab yang sama; file yang sudah terkirim dengan hasil belum pasti tidak boleh diimpor ulang dengan key baru sebelum mengecek server. Backend mendukung retail satu produk/varian per row, bukan import semua master/modifier/batch apotek. Perubahan klien API JSON/raw CSV belum diuji.
