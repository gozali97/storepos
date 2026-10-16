# B430 — 20 kontrak pajak & akses

**Implementasi selesai, testing ditunda; bukan verified.**

- [x] B430-01 Implementasi kontrak `DELETE /api/v1/invitations/{id}` — belum diuji.
- [x] B430-02 Implementasi kontrak `DELETE /api/v1/sales/{id}/receipt-links` — belum diuji.
- [x] B430-03 Implementasi kontrak `DELETE /api/v1/tax-rules/{id}` — belum diuji.
- [x] B430-04 Implementasi kontrak `GET /api/v1/categories/{id}/tax-rule` — belum diuji.
- [x] B430-05 Implementasi kontrak `GET /api/v1/invitations` — belum diuji.
- [x] B430-06 Implementasi kontrak `GET /api/v1/products/{id}/tax-rule` — belum diuji.
- [x] B430-07 Implementasi kontrak `GET /api/v1/sales/{id}/returns` — belum diuji.
- [x] B430-08 Implementasi kontrak `GET /api/v1/sales/{id}/void` — belum diuji.
- [x] B430-09 Implementasi kontrak `GET /api/v1/tax-rules` — belum diuji.
- [x] B430-10 Implementasi kontrak `GET /api/v1/tax-rules/{id}` — belum diuji.
- [x] B430-11 Implementasi kontrak `POST /api/v1/auth/logout` — belum diuji.
- [x] B430-12 Implementasi kontrak `POST /api/v1/invitations` — belum diuji.
- [x] B430-13 Implementasi kontrak `POST /api/v1/invitations/accept` — belum diuji.
- [x] B430-14 Implementasi kontrak `POST /api/v1/sales/{id}/receipt-links` — belum diuji.
- [x] B430-15 Implementasi kontrak `POST /api/v1/sales/{id}/reprints` — belum diuji.
- [x] B430-16 Implementasi kontrak `POST /api/v1/tax-rules` — belum diuji.
- [x] B430-17 Implementasi kontrak `POST /api/v1/tax-rules/{id}/restore` — belum diuji.
- [x] B430-18 Implementasi kontrak `PUT /api/v1/categories/{id}/tax-rule` — belum diuji.
- [x] B430-19 Implementasi kontrak `PUT /api/v1/products/{id}/tax-rule` — belum diuji.
- [x] B430-20 Implementasi kontrak `PUT /api/v1/tax-rules/{id}` — belum diuji.

Generator OpenAPI diperbarui untuk HTTP 204 tanpa content. Batch B430/B420 diberi implemented-not-tested; 100 kontrak batch lama tetap verified. Gofmt dan pembangkitan dokumen saja; tidak menjalankan go test, vet, build aplikasi, browser, atau migrasi.

Risiko: schema belum dicocokkan dengan respons HTTP nyata; operasi lain masih skeleton. Undangan tetap manual tanpa provider eksternal.
