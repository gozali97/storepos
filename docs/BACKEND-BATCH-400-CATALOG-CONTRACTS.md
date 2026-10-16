# B400 — 20 kontrak katalog & satuan

Lanjutan kontrak OpenAPI §16.4. Satu task = satu operasi API dengan schema terkurasi dan pengujian HTTP/PostgreSQL nyata, bukan fitur runtime baru.

- [x] B400-01: `DELETE /api/v1/variants/{id}/barcodes`
- [x] B400-02: `GET /api/v1/catalog/barcode`
- [x] B400-03: `GET /api/v1/catalog/search`
- [x] B400-04: `GET /api/v1/categories/archived`
- [x] B400-05: `GET /api/v1/products/{id}`
- [x] B400-06: `GET /api/v1/purchase-units/{id}`
- [x] B400-07: `GET /api/v1/variants/{id}`
- [x] B400-08: `GET /api/v1/variants/{id}/barcodes`
- [x] B400-09: `GET /api/v1/variants/{id}/units`
- [x] B400-10: `PATCH /api/v1/products/{id}/status`
- [x] B400-11: `PATCH /api/v1/variants/{id}/fractional`
- [x] B400-12: `PATCH /api/v1/variants/{id}/identity`
- [x] B400-13: `PATCH /api/v1/variants/{id}/price`
- [x] B400-14: `PATCH /api/v1/variants/{id}/status`
- [x] B400-15: `POST /api/v1/products`
- [x] B400-16: `POST /api/v1/purchase-units`
- [x] B400-17: `POST /api/v1/purchase-units/{id}/barcodes`
- [x] B400-18: `PUT /api/v1/products/{id}`
- [x] B400-19: `PUT /api/v1/purchase-units/{id}`
- [x] B400-20: `PUT /api/v1/purchase-units/{id}/sale-price`

Total kumulatif: 80 operasi verified. Menguji create/detail/edit/status produk, versi varian/harga/identitas/pecahan, barcode, create/detail/edit/harga/barcode satuan, lookup barcode/search, kategori arsip. Request/response memelihara uang string sen dan kuantitas string milli-unit.

Tambahan schema validator subset: map atribut string via additionalProperties, optional properties, nullable map, dan tes negatif atribut numerik/nominal numerik. If-Match harga satuan tetap opsional sesuai kompatibilitas handler.

Verifikasi: HTTP/PostgreSQL seluruh 20 operasi, tes overlap batch dan drift, YAML valid, go vet/full suite -race/build lulus. Frontend tidak berubah.

Batas: operasi lain masih skeleton. Validator subset bukan validator OpenAPI umum; browser/perangkat tidak diuji.
