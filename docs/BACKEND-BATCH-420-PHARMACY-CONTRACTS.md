# B420 — 20 kontrak apotek, opname, dan sesi

**Status: implementasi selesai, testing dilewati atas permintaan pengguna.** Bukan klaim verified.

- [x] B420-01 Implementasi kontrak `GET /api/v1/me/sessions` — belum diuji.
- [x] B420-02 Implementasi kontrak `GET /api/v1/pharmacy/batch-receipts` — belum diuji.
- [x] B420-03 Implementasi kontrak `GET /api/v1/pharmacy/batches` — belum diuji.
- [x] B420-04 Implementasi kontrak `GET /api/v1/pharmacy/batches/{id}/movements` — belum diuji.
- [x] B420-05 Implementasi kontrak `GET /api/v1/pharmacy/batches/{id}/trace` — belum diuji.
- [x] B420-06 Implementasi kontrak `GET /api/v1/pharmacy/fefo-preview` — belum diuji.
- [x] B420-07 Implementasi kontrak `GET /api/v1/pharmacy/products` — belum diuji.
- [x] B420-08 Implementasi kontrak `GET /api/v1/pharmacy/products/{id}` — belum diuji.
- [x] B420-09 Implementasi kontrak `GET /api/v1/stock/counts` — belum diuji.
- [x] B420-10 Implementasi kontrak `GET /api/v1/stock/counts/{id}` — belum diuji.
- [x] B420-11 Implementasi kontrak `POST /api/v1/me/sessions/revoke-others` — belum diuji.
- [x] B420-12 Implementasi kontrak `POST /api/v1/pharmacy/batch-receipts` — belum diuji.
- [x] B420-13 Implementasi kontrak `POST /api/v1/pharmacy/batches/{id}/count` — belum diuji.
- [x] B420-14 Implementasi kontrak `POST /api/v1/pharmacy/batches/{id}/dispose` — belum diuji.
- [x] B420-15 Implementasi kontrak `POST /api/v1/pharmacy/batches/{id}/release` — belum diuji.
- [x] B420-16 Implementasi kontrak `POST /api/v1/stock/counts` — belum diuji.
- [x] B420-17 Implementasi kontrak `POST /api/v1/stock/counts/{id}/post` — belum diuji.
- [x] B420-18 Implementasi kontrak `PUT /api/v1/auth/password` — belum diuji.
- [x] B420-19 Implementasi kontrak `PUT /api/v1/pharmacy/products/{id}` — belum diuji.
- [x] B420-20 Implementasi kontrak `PUT /api/v1/stock/counts/{id}/lines/{variant}` — belum diuji.

Schema request/response, query, versi, key idempotency, nominal string/milli-unit, nullable dan kredensial writeOnly ditambahkan. Kerangka tes HTTP telah ditulis sebelum permintaan skip testing, tetapi tidak dijalankan.

Dokumen OpenAPI dibangkitkan menggunakan fungsi generator melalui executable sementara (bukan go test), lalu executable sumber sementara dihapus dari repo. Hanya gofmt dijalankan; tidak menjalankan go test, go vet, build aplikasi, browser atau database migration.

Total: 100 operasi verified dari batch sebelumnya, 20 implemented-not-tested. Operasi lain tetap skeleton. Validator tes hanya subset schema, bukan validator OpenAPI umum.

Risiko: schema atau fixture baru bisa belum cocok dengan handler; perlu menjalankan suite kontrak dan full suite di tahap pengujian akhir. Tidak mengubah frontend/landing/runtime transaksi.
