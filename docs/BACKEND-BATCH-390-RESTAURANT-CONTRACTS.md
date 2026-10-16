# B390 — 20 kontrak konfigurasi restoran

Lanjutan task induk OpenAPI §16.4. Setiap operasi memiliki schema request bila ada, respons sukses/error envelope, header/autentikasi dan tes HTTP/PostgreSQL nyata.

- [x] B390-01: `DELETE /api/v1/restaurant/tables/{id}`
- [x] B390-02: `GET /api/v1/restaurant/areas`
- [x] B390-03: `GET /api/v1/restaurant/category-routes`
- [x] B390-04: `GET /api/v1/restaurant/menu/availability`
- [x] B390-05: `GET /api/v1/restaurant/products/{id}/modifiers`
- [x] B390-06: `GET /api/v1/restaurant/service-charge`
- [x] B390-07: `GET /api/v1/restaurant/stations`
- [x] B390-08: `GET /api/v1/restaurant/tables`
- [x] B390-09: `POST /api/v1/restaurant/modifier-groups`
- [x] B390-10: `POST /api/v1/restaurant/modifier-groups/{id}/options`
- [x] B390-11: `POST /api/v1/restaurant/stations`
- [x] B390-12: `POST /api/v1/restaurant/tables/{id}/clean`
- [x] B390-13: `POST /api/v1/restaurant/tables/{id}/restore`
- [x] B390-14: `PUT /api/v1/restaurant/category-routes/{id}`
- [x] B390-15: `PUT /api/v1/restaurant/menu/{id}/availability`
- [x] B390-16: `PUT /api/v1/restaurant/modifier-groups/{id}`
- [x] B390-17: `PUT /api/v1/restaurant/modifier-groups/{id}/options/{option}`
- [x] B390-18: `PUT /api/v1/restaurant/service-charge`
- [x] B390-19: `PUT /api/v1/restaurant/stations/{id}`
- [x] B390-20: `PUT /api/v1/restaurant/tables/{id}`

Total kumulatif: 60 operasi verified. Operasi lain tetap skeleton. Query branch_id wajib, pagination/filter arsip, If-Match 0 untuk konfigurasi baru, dan nominal modifier string integer sen didokumentasikan.

Verifikasi: TestTwentyRestaurantContractsMatchLiveHTTP, TestRestaurantContractBatchHasTwentyDistinctOperations, TestRestaurantContractStringMoneyAndQueryRequirements, seluruh tes batch lama/drift/operationId, vet/full race/build, parsing YAML.

Batas: validator tes hanya subset schema terkurasi, bukan validator OpenAPI umum. Tidak menambah fitur runtime atau frontend. Uji browser, perangkat dan operasi lain belum selesai.
