# ADR 0001 — Modular Monolith di Go

- Status: Diterima
- Tanggal: 2025-10-13
- Sumber: PRD §16.1, §16.2

## Konteks

Domain transaksi, stok, dan kas membutuhkan transaksi database atomik lintas modul (finalisasi penjualan mengubah saldo stok, pembayaran, shift, dan audit dalam satu commit). Tim di tahap awal perlu kecepatan iterasi dan unit deploy tunggal.

## Keputusan

Satu binary Go dengan dua mode: `api` dan `worker`. Batas modul internal ditegakkan oleh paket Go dan review kode. Komunikasi antar modul melalui interface Go, bukan akses langsung ke tabel modul lain. Satu PostgreSQL untuk semua modul.

## Konsekuensi

Positif:

- Transaksi DB tunggal untuk finalisasi (§13.5, §13.12).
- Rolling deploy sederhana (NFR-022).
- Observability satu tempat.

Negatif:

- Perlu disiplin batas modul agar tidak jadi big ball of mud.
- Perubahan infrastruktur per modul tidak dapat independen.

## Aturan modul

- Modul mendefinisikan API publiknya di `internal/<modul>/api.go`.
- Tabel modul tidak dirujuk oleh modul lain. Modul `reporting` boleh query baca saja.
- Lintas-modul via `uow` (unit of work) yang membawa transaksi `*sql.Tx` dan konteks tenant.
