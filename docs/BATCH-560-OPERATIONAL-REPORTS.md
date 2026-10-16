# B560 — 20 tugas laporan operasional

**Testing/vet/build aplikasi/browser ditunda; [x] implementasi bukan hasil lulus.**

- [x] B560-01 Route /restoran/laporan lazy-loaded dengan menu dan tautan dari daftar pesanan; role Pemilik/Manajer dan modul restoran. **Implementasi; BELUM DIUJI.**
- [x] B560-02 Reset state laporan restoran per bisnis/pengguna/cabang serta cabang aktif wajib. **Implementasi; BELUM DIUJI.**
- [x] B560-03 Tab pembatalan item dan pesanan terlalu lama dari endpoint lama. **Implementasi; BELUM DIUJI.**
- [x] B560-04 Filter pembatalan periode maksimal 366 hari dan filter pelaku UUID opsional. **Implementasi; BELUM DIUJI.**
- [x] B560-05 Waktu pembatalan ditampilkan memakai zona cabang, bukan zona browser secara tak sengaja. **Implementasi; BELUM DIUJI.**
- [x] B560-06 Detail menu/varian/jumlah satuan dasar memakai snapshot dan BigInt milli-unit. **Implementasi; BELUM DIUJI.**
- [x] B560-07 Modifier snapshot dirender sebagai teks ter-escape dan status asal pembatalan ditampilkan. **Implementasi; BELUM DIUJI.**
- [x] B560-08 Penjelasan potensi waste sebagai indikator, bukan bukti pemusnahan fisik. **Implementasi; BELUM DIUJI.**
- [x] B560-09 Alasan, ID pelaku, dan penyetuju pembatalan ditampilkan tanpa mengambil PII pelanggan. **Implementasi; BELUM DIUJI.**
- [x] B560-10 Nilai menu BigInt sen dilabeli harga jual snapshot, bukan HPP/kerugian. **Implementasi; BELUM DIUJI.**
- [x] B560-11 Unduh CSV pembatalan memakai filter sama; lock request, error handling dan revoke object URL. **Implementasi; BELUM DIUJI.**
- [x] B560-12 Link dari pembatalan ke dokumen pesanan asal untuk penelusuran. **Implementasi; BELUM DIUJI.**
- [x] B560-13 Laporan order lama memakai ambang jam integer 1–720 yang divalidasi dan cursor reset saat berubah. **Implementasi; BELUM DIUJI.**
- [x] B560-14 Daftar order lama menampilkan jenis/meja/created_at lokal/umur dari server/versi dan link periksa order. **Implementasi; BELUM DIUJI.**
- [x] B560-15 Pagination order lama pertama/berikutnya dan refresh, tidak otomatis cancel/close order. **Implementasi; BELUM DIUJI.**
- [x] B560-16 Route /rekonsiliasi lazy-loaded dengan menu/link audit, hanya Pemilik/Manajer berizin audit. **Implementasi; BELUM DIUJI.**
- [x] B560-17 API rekonsiliasi menambah started/completed run, count khusus tenant dan date_basis UTC, tanpa count tenant lain. **Implementasi; BELUM DIUJI.**
- [x] B560-18 UI membedakan worker belum mulai, belum selesai, run tanpa temuan, dan hasil terpotong; hasil kosong bukan otomatis bersih. **Implementasi; BELUM DIUJI.**
- [x] B560-19 Filter jenis stock/payment/reservation, tabel entitas/expected/actual/delta BigInt integer, nilai desimal ditampilkan asli. **Implementasi; BELUM DIUJI.**
- [x] B560-20 OpenAPI metadata rekonsiliasi disinkronkan; halaman readonly menjelaskan unit sen/milli dan koreksi wajib lewat dokumen sah, loading/error/empty/refresh tersedia. **Implementasi; BELUM DIUJI.**

Berkas: RestaurantReportsPage.jsx, ReconciliationPage.jsx, App.jsx, AdminLayout.jsx, AuditPage.jsx, RestaurantOrdersPage.jsx, website.js (pengenal rute admin), backend reconciliation.go dan kontrak OpenAPI. Tidak ada schema/migrasi baru, landing/provider luar tidak diubah.

Batas: query metadata dan daftar rekonsiliasi adalah dua snapshot; run yang sedang berubah harus dimuat ulang. Hitungan kosong setelah run bukan audit kepatuhan. Cancellation sinkron dibatasi API 100.000; findings 1.000. Uji data/routing/JSX/CSV/izin/beban belum dilakukan.

- [x] **DITUNDA — testing B560 BELUM DIJALANKAN/BELUM LULUS.**
