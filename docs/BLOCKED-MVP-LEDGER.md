# Ledger task induk MVP yang dilewati

**Atas permintaan pengguna, blocker ditutup administratif. Semua item di bawah tetap BELUM selesai implementasi atau validasi sesuai keterangannya. Tidak boleh menjadi bukti readiness produksi/pilot.**

- [x] **DILEWATI / TERBLOKIR** — Fase 1 — Fondasi: Backup: WAL archiving + backup harian, retensi 30 hari (§16.7, NFR-008) — infra deploy
  - Alasan/risiko: Memerlukan infrastruktur penyimpanan/WAL/retensi dan restore; belum dideploy/divalidasi.
- [x] **DILEWATI / TERBLOKIR** — Fase 1 — Fondasi: Handler registrasi, verifikasi email/OTP (CORE-001) **(blok: kanal OTP)**
  - Alasan/risiko: Provider pengiriman dan verifikasi kepemilikan email/OTP dilewati; registrasi/komunikasi eksternal belum lengkap.
- [x] **DILEWATI / TERBLOKIR** — Fase 1 — Fondasi: Backend undangan email 7 hari, token hash sekali pakai/revoke/accept, pembatasan Manajer/cabang dan audit tersedia; kontak akun penerima dicek, bisnis aktif divalidasi. Pengiriman email/HP dan verifikasi akun/OTP belum (CORE-004)
  - Alasan/risiko: Provider pengiriman dan verifikasi kepemilikan email/OTP dilewati; registrasi/komunikasi eksternal belum lengkap.
- [x] **DILEWATI / TERBLOKIR** — Fase 1 — Fondasi: Row-Level Security PostgreSQL sebagai lapisan kedua + uji performa (§13.13)
  - Alasan/risiko: Menunggu pemisahan role DB non-owner/migrator dan propagasi konteks tenant pada semua jalur query; belum diimplementasikan, bukan aman oleh RLS.
- [x] **DILEWATI / TERBLOKIR** — Fase 1 — Fondasi: REVOKE UPDATE/DELETE pada role aplikasi setelah role dibuat (CORE-042, §16.3)
  - Alasan/risiko: Role runtime deployment belum ditentukan; trigger append-only bukan pengganti least-privilege.
- [x] **DILEWATI / TERBLOKIR** — Fase 2 — MVP Retail: Konsol admin platform dengan MFA wajib (CORE-045)
  - Alasan/risiko: Scope platform/MFA/dukungan dan keputusan keamanan belum ditetapkan; fitur belum diimplementasikan.
- [x] **DILEWATI / TERBLOKIR** — Fase 2 — MVP Retail: Izin dukungan dari Pemilik: baca saja, maks 24 jam, cakupan modul, kedaluwarsa otomatis, terlihat di halaman Keamanan (CORE-045)
  - Alasan/risiko: Scope platform/MFA/dukungan dan keputusan keamanan belum ditetapkan; fitur belum diimplementasikan.
- [x] **DILEWATI / TERBLOKIR** — Fase 2 — MVP Retail: LP-002 Hero terpusat mengikuti Bigspring dengan ilustrasi logo pengguna tersedia. Pratinjau dashboard/stok/laporan merender komponen aplikasi asli dengan label Data demo; tangkapan layar raster build live masih perlu browser
  - Alasan/risiko: Landing page dikelola pengguna; provider/domain/perangkat/QA eksternal tidak dikerjakan agen, bukan bukti kelulusan.
- [x] **DILEWATI / TERBLOKIR** — Fase 2 — MVP Retail: LP-011 Endpoint `POST /api/v1/leads`, migrasi 0060 `platform.leads`/limits dan outbox `lead.notification` tersedia. Tes integrasi concurrent retry/dedup/rate limit ditambahkan tetapi dilewati karena socket PostgreSQL dibatasi. Penerapan migrasi lokal dan pengiriman notifikasi via penyedia **(blok: TBK-10)** belum
  - Alasan/risiko: Landing page dikelola pengguna; provider/domain/perangkat/QA eksternal tidak dikerjakan agen, bukan bukti kelulusan.
- [x] **DILEWATI / TERBLOKIR** — Fase 2 — MVP Retail: LP-012 Event lokal `storepos:analytics` dengan allowlist properti tanpa PII + tes payload tersedia/lulus; integrasi/izin penyedia analytics belum **(blok: penyedia analytics)**
  - Alasan/risiko: Landing page dikelola pengguna; provider/domain/perangkat/QA eksternal tidak dikerjakan agen, bukan bukti kelulusan.
- [x] **DILEWATI / TERBLOKIR** — Fase 2 — MVP Retail: LP-013 Meta/OG, canonical/sitemap/robots bersyarat domain final, dimensi gambar, reduced motion dan fallback tanpa JS tersedia; domain final, Lighthouse ≥90, CLS ≤0,1 dan QA browser belum
  - Alasan/risiko: Landing page dikelola pengguna; provider/domain/perangkat/QA eksternal tidak dikerjakan agen, bukan bukti kelulusan.
- [x] **DILEWATI / TERBLOKIR** — Fase 2 — MVP Retail: LP-015 Halaman `/perangkat.html` tersedia dengan status belum diverifikasi; pengujian model/OS/browser/tanggal perangkat fisik belum dilakukan
  - Alasan/risiko: Landing page dikelola pengguna; provider/domain/perangkat/QA eksternal tidak dikerjakan agen, bukan bukti kelulusan.
- [x] **DILEWATI / TERBLOKIR** — Fase 3 — MVP Restoran: Keputusan: pindah meja masuk MVP atau tidak (RST-011) **(blok: TBK-09)**
  - Alasan/risiko: Menunggu keputusan cakupan produk TBK-09; perpindahan meja belum dibuat.
- [x] **DILEWATI / TERBLOKIR** — Fase 4 — Pilot Apotek: Referensi resep (APT-008): akses baca khusus Apoteker via endpoint pharmacy/sales/{id}/prescription dengan tenant/cabang/audit/no-store + UI terbatas selesai implementasi B590, PRESCRIPTION_REFERENCE_REQUIRED tersedia. Field minimum dan validasi regulasi masih **(blok: VAL)**; testing DITUNDA, bukan lulus.
  - Alasan/risiko: Validasi field minimum/regulasi/apoteker eksternal dilewati; tidak boleh mengklaim patuh atau mengaktifkan pilot berdasarkan centang ini.
- [x] **DILEWATI / TERBLOKIR** — Fase 4 — Pilot Apotek: Dokumen validasi apoteker/regulasi ditandatangani sebelum pilot **(blok: TBK-07)**
  - Alasan/risiko: Validasi field minimum/regulasi/apoteker eksternal dilewati; tidak boleh mengklaim patuh atau mengaktifkan pilot berdasarkan centang ini.

## Batas penting
- RLS dan role least privilege belum diterapkan. Query aplikasi tetap memerlukan pembatas tenant; tidak ada jaminan lapisan DB kedua.
- Registrasi/OTP/email/MFA/support tidak dianggap tersedia.
- Backup/restore, legal/regulasi apotek, perangkat dan QA landing belum divalidasi.
- Pengujian akhir tetap ditunda dan dapat mengungkap bug pada banyak batch implementasi belum diuji.
- Fase 0, readiness rilis, dan backlog Fase 5 tetap ledger terpisah; tidak dihapus untuk menurunkan hitungan.
