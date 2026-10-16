# ADR 0004 — Runner migrasi Go dengan SQL embedded

Status: diterima

## Keputusan

StorePOS memakai runner yang sudah ada di `backend/internal/platform/migrate`, bukan tool migrasi tambahan. Berkas `sql/NNNN_nama.up.sql` dan `.down.sql` di-embed ke binary. Sumber kebenaran berada di `backend/internal/platform/migrate/sql`.

`storepos migrate up`, `down`, dan `status` dipanggil lewat target Makefile. `up` menerapkan versi yang belum tercatat di `schema_migrations`; `down` membalik satu versi terakhir per pemanggilan. Setiap penerapan memakai transaksi database.

## Alasan

API, worker, dan CLI memakai binary serta konfigurasi PostgreSQL yang sama. Tidak perlu dependency/installer migrasi terpisah atau salinan SQL di direktori deployment. Tes runner yang ada memeriksa discovery berkas, pasangan up/down, penomoran ganda, serta penerapan dan rollback.

## Batas operasional

Jalankan migrasi sebagai langkah deployment tersendiri oleh satu migrator, bukan oleh setiap replica API. Nomor versi harus unik. Jangan mengedit migrasi yang sudah diterapkan; tambahkan versi baru. Migrasi down tetap membutuhkan pemeriksaan operator karena rollback struktur bisa menghilangkan data.

Role migrator dan role runtime PostgreSQL serta pengujian recovery deployment merupakan pekerjaan deployment terpisah, tidak diselesaikan oleh ADR ini.
