# B440 — 20 kontrak draft dan alur pesanan

**Implementasi selesai; testing ditunda, bukan lulus.**

- [x] B440-01 Implementasi kontrak `DELETE /api/v1/restaurant/orders/{id}/lines/{line}` — BELUM DIUJI.
- [x] B440-02 Implementasi kontrak `GET /api/v1/drafts` — BELUM DIUJI.
- [x] B440-03 Implementasi kontrak `GET /api/v1/restaurant/kitchen/tickets` — BELUM DIUJI.
- [x] B440-04 Implementasi kontrak `GET /api/v1/restaurant/orders` — BELUM DIUJI.
- [x] B440-05 Implementasi kontrak `GET /api/v1/restaurant/orders/{id}` — BELUM DIUJI.
- [x] B440-06 Implementasi kontrak `GET /api/v1/restaurant/orders/{id}/bill` — BELUM DIUJI.
- [x] B440-07 Implementasi kontrak `GET /api/v1/restaurant/orders/{id}/discounts` — BELUM DIUJI.
- [x] B440-08 Implementasi kontrak `GET /api/v1/restaurant/orders/{id}/lines` — BELUM DIUJI.
- [x] B440-09 Implementasi kontrak `POST /api/v1/drafts` — BELUM DIUJI.
- [x] B440-10 Implementasi kontrak `POST /api/v1/restaurant/orders` — BELUM DIUJI.
- [x] B440-11 Implementasi kontrak `POST /api/v1/restaurant/orders/{id}/cancel` — BELUM DIUJI.
- [x] B440-12 Implementasi kontrak `POST /api/v1/restaurant/orders/{id}/checkout` — BELUM DIUJI.
- [x] B440-13 Implementasi kontrak `POST /api/v1/restaurant/orders/{id}/lines` — BELUM DIUJI.
- [x] B440-14 Implementasi kontrak `POST /api/v1/restaurant/orders/{id}/lines/{line}/cancel` — BELUM DIUJI.
- [x] B440-15 Implementasi kontrak `POST /api/v1/restaurant/orders/{id}/send` — BELUM DIUJI.
- [x] B440-16 Implementasi kontrak `PUT /api/v1/drafts/{id}` — BELUM DIUJI.
- [x] B440-17 Implementasi kontrak `PUT /api/v1/restaurant/orders/{id}` — BELUM DIUJI.
- [x] B440-18 Implementasi kontrak `PUT /api/v1/restaurant/orders/{id}/discount` — BELUM DIUJI.
- [x] B440-19 Implementasi kontrak `PUT /api/v1/restaurant/orders/{id}/lines/{line}` — BELUM DIUJI.
- [x] B440-20 Implementasi kontrak `PUT /api/v1/restaurant/orders/{id}/lines/{line}/discount` — BELUM DIUJI.

Cakupan: request/response, nullable, versi, query cabang/stasiun/cursor, idempotency send/checkout, pembayaran string sen, snapshot modifier, draft hold/resume, diskon dan persetujuan. POST order dapat 201 (baru) atau 200 (resume).

Tidak menjalankan testing, vet, build aplikasi, browser, atau migrasi. Hanya gofmt dan generator dokumen lewat executable sementara; sumber sementara dihapus setelah selesai.

Status: 100 operasi verified dari batch lama; 60 implemented-not-tested (B420/B430/B440). Sisanya skeleton. Task induk OpenAPI tetap terbuka. Frontend/runtime/landing tidak diubah.
