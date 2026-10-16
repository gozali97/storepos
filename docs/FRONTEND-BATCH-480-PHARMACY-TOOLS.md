# B480 — 20 implementasi operasional frontend apotek

**Tidak menjalankan testing, vet, build aplikasi, browser atau migrasi. Tanda [x] hanya implementasi.**

- [x] B480-01 Halaman `/apotek/kelola` lazy-loaded, rute aplikasi/preview dan menu apotek, tautan dari Operasi Batch. **Implementasi; BELUM DIUJI.**
- [x] B480-02 Pembatasan jenis usaha/peran dan cabang; state direset per bisnis/pengguna/cabang agar data lama tidak terbawa. **Implementasi; BELUM DIUJI.**
- [x] B480-03 Tab laporan batch, riwayat penerimaan, dan identitas obat dengan akses berdasarkan izin operasional. **Implementasi; BELUM DIUJI.**
- [x] B480-04 Laporan batch JSON di layar dari API resmi, menampilkan produk/SKU/batch/ED/status. **Implementasi; BELUM DIUJI.**
- [x] B480-05 Kontrol periode pergerakan dengan validasi tanggal kosong/urutan tanggal. **Implementasi; BELUM DIUJI.**
- [x] B480-06 Filter semua/mendekati ED/kedaluwarsa/karantina dan ambang 0–3650 hari. **Implementasi; BELUM DIUJI.**
- [x] B480-07 Penjelasan saldo/nilai saat ini versus pergerakan periode agar bukan klaim saldo historis. **Implementasi; BELUM DIUJI.**
- [x] B480-08 Kolom saldo dan pergerakan memakai BigInt milli-unit, bukan konversi float. **Implementasi; BELUM DIUJI.**
- [x] B480-09 Kolom supplier terakhir, biaya/unit dan estimasi nilai hanya dirender jika server cost_visible. **Implementasi; BELUM DIUJI.**
- [x] B480-10 Biaya yang tidak diketahui ditampilkan sebagai Belum diketahui, bukan nilai nol. **Implementasi; BELUM DIUJI.**
- [x] B480-11 Download CSV memakai periode/filter/ambang yang sama dengan laporan layar, dengan error dan busy state. **Implementasi; BELUM DIUJI.**
- [x] B480-12 Tombol refresh laporan, status loading/error/empty memakai ResourceState proyek. **Implementasi; BELUM DIUJI.**
- [x] B480-13 Riwayat penerimaan batch cabang menampilkan faktur, nomor batch, ED, status dan jumlah. **Implementasi; BELUM DIUJI.**
- [x] B480-14 Pagination cursor riwayat penerimaan dengan navigasi halaman pertama/berikutnya. **Implementasi; BELUM DIUJI.**
- [x] B480-15 Buka kartu gerakan dari laporan atau riwayat; cursor reset ketika batch berubah. **Implementasi; BELUM DIUJI.**
- [x] B480-16 Kartu gerakan berhalaman menampilkan waktu zona cabang, jenis/status, delta, saldo sesudah, alasan dan ref_id. **Implementasi; BELUM DIUJI.**
- [x] B480-17 Penelusuran penjualan/retur dari kartu batch, menampilkan status, qty dan alasan override tanpa PII pelanggan; batas 200 baris dinyatakan. **Implementasi; BELUM DIUJI.**
- [x] B480-18 Daftar identitas obat berhalaman dengan golongan/wajib resep/pembatasan khusus; pencarian produk/SKU untuk menambah identitas. **Implementasi; BELUM DIUJI.**
- [x] B480-19 Editor identitas memuat versi PRODUCT dan detail obat; record belum ada (404) membuka form kosong tanpa menelan error lain. **Implementasi; BELUM DIUJI.**
- [x] B480-20 Simpan identitas/flag resep/pembatasan memakai If-Match versi produk, proteksi konflik 409 (wajib muat ulang), label izin edar/golongan belum diverifikasi. **Implementasi; BELUM DIUJI.**

Berkas: PharmacyTools.jsx, PharmacyPage.jsx, App.jsx, AdminLayout.jsx, website.js (hanya pengenal rute aplikasi; landing tidak diubah). Semua data/mutasi menggunakan endpoint backend yang sudah ada.

Risiko: JSX/routing/API baru belum diverifikasi build/browser; legal/regulasi/izin edar tidak diverifikasi. Nilai batch estimasi dari penerimaan, bukan HPP inti. Trace penjualan server dibatasi 200; tabel laporan sinkron dibatasi API, belum virtualized. Editor memakai versi produk dan detail obat dua request terpisah, 409 tetap sumber kebenaran konflik.

- [x] **DITUNDA — testing B480 BELUM DIJALANKAN/BELUM LULUS.**
