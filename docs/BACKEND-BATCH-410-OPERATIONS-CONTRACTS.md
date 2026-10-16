# B410 — 20 kontrak operasional

Satu task = satu operasi API yang schema dan respons HTTP/PostgreSQL-nya diuji. Ini bagian task induk OpenAPI §16.4, bukan fitur runtime baru.

- [x] B410-01: `GET /api/v1/expenses`
- [x] B410-02: `GET /api/v1/expenses/{id}`
- [x] B410-03: `GET /api/v1/goods-receipts`
- [x] B410-04: `GET /api/v1/goods-receipts/{id}`
- [x] B410-05: `GET /api/v1/settings/operational-policy`
- [x] B410-06: `GET /api/v1/shifts/open`
- [x] B410-07: `GET /api/v1/shifts/{id}/summary`
- [x] B410-08: `GET /api/v1/stock/reorder`
- [x] B410-09: `POST /api/v1/expenses`
- [x] B410-10: `POST /api/v1/goods-receipts`
- [x] B410-11: `POST /api/v1/goods-receipts/{id}/post`
- [x] B410-12: `POST /api/v1/shifts/cash`
- [x] B410-13: `POST /api/v1/shifts/close`
- [x] B410-14: `POST /api/v1/shifts/open`
- [x] B410-15: `POST /api/v1/stock/receipts`
- [x] B410-16: `PUT /api/v1/goods-receipts/{id}`
- [x] B410-17: `PUT /api/v1/memberships/me/pin`
- [x] B410-18: `PUT /api/v1/settings/operational-policy`
- [x] B410-19: `PUT /api/v1/stock/minimums/{variant}`
- [x] B410-20: `PUT /api/v1/stock/variants/{id}/opening-cost`

Total: 100 operasi verified. Kebijakan operasional 2, shift/PIN 6, penerimaan 5, pengeluaran 3, stok 4.

Kontrak mencakup status 200/201, response nullable sebelum shift terbuka, draft/posted detail, nominal string sen/milli-unit, ringkasan shift, query periode/cabang/cursor, If-Match versi 0 stok minimum, key idempotency wajib vs opsional untuk klien lama.

Verifikasi: TestTwentyOperationalContractsMatchLiveHTTP, TestOperationalContractBatchHasTwentyDistinctOperations, TestOperationalMoneyAndIdempotencyContracts, tes batch sebelumnya/drift, go vet/full suite -race/build, parse YAML.

Batas: validator schema subset, bukan validator OpenAPI umum. Operasi lain skeleton; frontend dan data runtime tidak diubah. Uji browser/perangkat belum.
