# B540 — Keamanan akun & riwayat ekspor

Migrasi lokal: migrate-up + status dijalankan, seluruh versi sampai 0065 applied. Tidak ada migrasi baru B540.

## Implementasi
- [x] /keamanan lazy-loaded, daftar sesi sendiri, current session, browser/IP dan timestamp, cursor, refresh manual/60 detik, cleanup timer.
- [x] Cabut seluruh sesi lain dengan checkbox konfirmasi dan request lock, tidak mencabut sesi saat ini. Error/network menampilkan status belum pasti dan refresh daftar.
- [x] /ekspor lazy-loaded, riwayat export user/bisnis, pagination, job status dan expiry, detail error/count, download API blob dengan permission/retensi server, revoke object URL.
- [x] Menu/link Settings/Reports dan pengenal route admin/preview disambungkan.
- [x] DITUNDA: testing/vet/build aplikasi/browser BELUM DIJALANKAN/BELUM LULUS.

Batas: tidak memverifikasi device asli dari user-agent/IP. Ekspor membutuhkan worker; tidak menambahkan storage/provider luar. Tidak menyentuh landing/backend runtime.
