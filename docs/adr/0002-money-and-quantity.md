# ADR 0002 — Representasi Uang & Kuantitas

- Status: Diterima
- Sumber: PRD §13.7

## Keputusan

Uang disimpan sebagai `BIGINT` sen di PostgreSQL dan `int64` di Go. Kuantitas disimpan sebagai `NUMERIC(14,3)` di PostgreSQL dan `int64` milli-unit (×1000) di Go. Floating point dilarang untuk uang dan kuantitas di seluruh stack.

Frontend menerima/mengirim uang sebagai **string integer sen** untuk menghindari kehilangan presisi di JavaScript (`Number.MAX_SAFE_INTEGER` cukup, tapi tetap dikirim sebagai string demi konsistensi serialisasi).

## Pembulatan

Half-up ke rupiah penuh (100 sen) pada titik yang ditentukan di §13.8. Berkas `pricing/testdata/vectors.json` berisi kasus input → output. Mesin Go dan mesin React menjalankan vektor yang sama di CI.

## Konsekuensi

- Semua kolom uang di skema diberi akhiran `_cents` kecuali dinyatakan lain.
- Konversi tampilan dilakukan di satu utilitas di frontend (`formatIDR`) dan satu paket Go (`money`).
- Serialisasi JSON pakai string untuk kolom `_cents`.
