# B580 — konfigurasi metode manual dan terminal

- [x] Implementasi API config/version/audit/permission dan default cash/qris/card/transfer.
- [x] Penjualan baru menolak metode nonaktif; refund lama tetap metode asal.
- [x] UI pengaturan dan selector kasir/restoran + daftar terminal nonaktif berhalaman/aktif ulang.
- [x] Kontrak API dua operasi implemented-not-tested.
- [x] DITUNDA: test/vet/build aplikasi/browser belum dijalankan/belum lulus.

**Migrasi 0066 belum diterapkan. Terapkan sebelum menjalankan runtime baru**, karena finalisasi sekarang membaca payment_methods_version. Tidak ada payment gateway/custom-code baru; nama display hanya untuk form penjualan, histori/report memakai kode stabil. Konfigurasi salah dari SQL manual ditolak loader, bukan fallback diam-diam. Tidak ada jaminan kelulusan QA.

Sisa task induk lainnya terblokir provider/infra/regulasi/landing atau keputusan produk; tidak ditutup hanya untuk memenuhi target 5.
