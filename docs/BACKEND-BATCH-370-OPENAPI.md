# Batch B370 — 20 kontrak operasi OpenAPI

Setiap task adalah satu operasi API dengan schema request (jika ada), response sukses, header/auth, dan pemeriksaan respons HTTP/PostgreSQL nyata. Tidak menganggap 20 operasi ini sama dengan 20 task induk PRD.

- [x] B370-01: `DELETE /api/v1/onboarding/demo`
- [x] B370-02: `DELETE /api/v1/restaurant/areas/{id}`
- [x] B370-03: `GET /api/v1/me/profile`
- [x] B370-04: `GET /api/v1/onboarding`
- [x] B370-05: `GET /api/v1/settings/business-profile`
- [x] B370-06: `GET /api/v1/settings/business-type`
- [x] B370-07: `GET /api/v1/settings/tax`
- [x] B370-08: `GET /api/v1/variants/{id}/unit-defaults`
- [x] B370-09: `POST /api/v1/onboarding/demo`
- [x] B370-10: `POST /api/v1/onboarding/events`
- [x] B370-11: `POST /api/v1/restaurant/areas`
- [x] B370-12: `POST /api/v1/restaurant/areas/{id}/restore`
- [x] B370-13: `POST /api/v1/restaurant/tables`
- [x] B370-14: `PUT /api/v1/me/profile`
- [x] B370-15: `PUT /api/v1/onboarding/dismiss`
- [x] B370-16: `PUT /api/v1/restaurant/areas/{id}`
- [x] B370-17: `PUT /api/v1/settings/business-profile`
- [x] B370-18: `PUT /api/v1/settings/business-type`
- [x] B370-19: `PUT /api/v1/settings/tax`
- [x] B370-20: `PUT /api/v1/variants/{id}/unit-defaults`

Verifikasi: TestTwentyOpenAPIContractsMatchLiveHTTP, TestCuratedContractsReferenceRegisteredRoutes, TestContractValidatorRejectsSchemaDrift, TestOpenAPIMatchesRegisteredRoutes, TestOperationIDsAreUnique; go vet/full suite -race/build lulus. YAML 3.0.3 bisa diparse; 20 operasi verified, operasi lain skeleton.

Batas: validator tes hanya mendukung subset schema yang dipakai batch ini, bukan validator OpenAPI umum. Error response diseragamkan, daftar error spesifik setiap handler belum semuanya dikurasi. Database/HTTP memakai lingkungan lokal, bukan browser/perangkat nyata.
