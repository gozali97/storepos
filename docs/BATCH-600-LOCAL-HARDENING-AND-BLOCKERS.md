# B600 — perbaikan lokal & penutupan blocker

- [x] Menutup administratif blocker MVP dengan label jelas dan ledger, bukan mengaku implementasi selesai.
- [x] Lock/cross-check hash dan status user sebelum perubahan password commit untuk mencegah lost-update.
- [x] Cast numeric sebelum qty*cost batch report, bukan sesudah overflow terjadi.
- [x] Scope state pengaturan organisasi per user/bisnis/cabang; validasi nama metode pembayaran UTF-8/nonblank.
- [x] DITUNDA: testing/vet/build aplikasi/browser belum dijalankan/belum lulus.

Tidak ada migrasi baru. Perbaikan berdasarkan pembacaan kode, belum pembuktian runtime. Sisa blocker dan release gates tetap ada di docs/BLOCKED-MVP-LEDGER.md. Tidak siap produksi hanya karena checklist selesai administratif.
