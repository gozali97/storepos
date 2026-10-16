# B460 — 20 kontrak transaksi/stok/akun

**Implementasi selesai; testing ditunda, bukan verified.**

- [x] B460-01 Implementasi kontrak `DELETE /api/v1/customers/{id}` — BELUM DIUJI.
- [x] B460-02 Implementasi kontrak `DELETE /api/v1/suppliers/{id}` — BELUM DIUJI.
- [x] B460-03 Implementasi kontrak `GET /api/v1/me` — BELUM DIUJI.
- [x] B460-04 Implementasi kontrak `GET /api/v1/memberships` — BELUM DIUJI.
- [x] B460-05 Implementasi kontrak `GET /api/v1/reports/products` — BELUM DIUJI.
- [x] B460-06 Implementasi kontrak `GET /api/v1/reports/stock` — BELUM DIUJI.
- [x] B460-07 Implementasi kontrak `GET /api/v1/reports/stock/batches` — BELUM DIUJI.
- [x] B460-08 Implementasi kontrak `GET /api/v1/sales` — BELUM DIUJI.
- [x] B460-09 Implementasi kontrak `GET /api/v1/sales/{id}` — BELUM DIUJI.
- [x] B460-10 Implementasi kontrak `GET /api/v1/stock/card` — BELUM DIUJI.
- [x] B460-11 Implementasi kontrak `GET /api/v1/workspace` — BELUM DIUJI.
- [x] B460-12 Implementasi kontrak `POST /api/v1/auth/login` — BELUM DIUJI.
- [x] B460-13 Implementasi kontrak `POST /api/v1/catalog/variant-combinations` — BELUM DIUJI.
- [x] B460-14 Implementasi kontrak `POST /api/v1/public/receipt` — BELUM DIUJI.
- [x] B460-15 Implementasi kontrak `POST /api/v1/sales/finalize` — BELUM DIUJI.
- [x] B460-16 Implementasi kontrak `POST /api/v1/sales/{id}/returns` — BELUM DIUJI.
- [x] B460-17 Implementasi kontrak `POST /api/v1/sales/{id}/void` — BELUM DIUJI.
- [x] B460-18 Implementasi kontrak `POST /api/v1/stock/adjustments` — BELUM DIUJI.
- [x] B460-19 Implementasi kontrak `PUT /api/v1/memberships/{id}` — BELUM DIUJI.
- [x] B460-20 Implementasi kontrak `PUT /api/v1/session/business` — BELUM DIUJI.

Schema request/response, idempotency transaksi/koreksi, versi anggota, query cabang/periode/customer/cursor, nullable snapshot/harga historis, uang string, dan report CSV ditambahkan. Status anggota sesuai handler active/disabled.

Tidak menjalankan test, vet, build aplikasi, browser, atau migrasi. Hanya gofmt dan generator dokumen lewat executable sementara; direktori sementara dihapus. Tidak mengubah frontend, landing page, atau runtime transaksi.

Total: 100 operasi verified (riwayat lama), 100 implemented-not-tested (B420–B460). Operasi lain skeleton. Task induk OpenAPI tetap terbuka karena belum seluruh operasi lengkap/tervalidasi.
