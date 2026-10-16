# B380 — 20 kontrak master data API

Lanjutan task induk kontrak OpenAPI (§16.4). Masing-masing operasi memiliki schema request bila menerima JSON, schema response sukses, header/auth, dan tes respons HTTP/PostgreSQL nyata.

- [x] B380-01: `DELETE /api/v1/categories/{id}`
- [x] B380-02: `GET /api/v1/branches/{id}`
- [x] B380-03: `GET /api/v1/categories`
- [x] B380-04: `GET /api/v1/categories/{id}`
- [x] B380-05: `GET /api/v1/customers`
- [x] B380-06: `GET /api/v1/customers/{id}`
- [x] B380-07: `GET /api/v1/registers/{id}`
- [x] B380-08: `GET /api/v1/suppliers`
- [x] B380-09: `GET /api/v1/suppliers/{id}`
- [x] B380-10: `PATCH /api/v1/registers/{id}/status`
- [x] B380-11: `POST /api/v1/branches`
- [x] B380-12: `POST /api/v1/branches/{id}/registers`
- [x] B380-13: `POST /api/v1/categories`
- [x] B380-14: `POST /api/v1/categories/{id}/restore`
- [x] B380-15: `POST /api/v1/customers`
- [x] B380-16: `POST /api/v1/suppliers`
- [x] B380-17: `PUT /api/v1/branches/{id}`
- [x] B380-18: `PUT /api/v1/categories/{id}`
- [x] B380-19: `PUT /api/v1/customers/{id}`
- [x] B380-20: `PUT /api/v1/suppliers/{id}`

Total kumulatif: 40 operasi verified. Operasi lain tetap skeleton, task induk tidak dianggap selesai. Query opsional q/after pada daftar pelanggan/supplier didokumentasikan.

Verifikasi: TestTwentyMasterContractsMatchLiveHTTP, TestMasterContractBatchHasTwentyDistinctOperations, suite kontrak batch awal, drift OpenAPI/operationId, go vet, full suite -race, go build. YAML berhasil diparse.

Batas: validator tes subset schema, bukan validator OpenAPI umum. Frontend dan data runtime tidak diubah; browser/perangkat belum diuji.
