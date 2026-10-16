# B500 — 20 fungsi riwayat & koreksi shift

**Testing/vet/build aplikasi/browser dan migrasi database tidak dijalankan.** Tanda [x] hanya implementasi.

- [x] B500-01 Model shift_correction_notes dengan FK tenant+shift dan batas isi catatan (migrasi 0065). **Implementasi; BELUM DIUJI.**
- [x] B500-02 Trigger catatan append-only: UPDATE/DELETE/TRUNCATE ditolak, angka/status shift tidak disentuh. **Implementasi; BELUM DIUJI.**
- [x] B500-03 Indeks riwayat shift per tenant/cabang/waktu dan indeks cursor catatan. **Implementasi; BELUM DIUJI.**
- [x] B500-04 Endpoint GET /shifts: filter periode berdasarkan tanggal lokal cabang, maksimal satu tahun. **Implementasi; BELUM DIUJI.**
- [x] B500-05 Filter status all/open/closed serta terminal yang diverifikasi milik tenant/cabang. **Implementasi; BELUM DIUJI.**
- [x] B500-06 Pembatasan daftar shift sendiri bagi kasir/apoteker versus semua shift cabang berizin untuk pemilik/manajer. **Implementasi; BELUM DIUJI.**
- [x] B500-07 Pagination daftar shift dengan cursor timestamp+ID dan validasi cursor pada seluruh filter. **Implementasi; BELUM DIUJI.**
- [x] B500-08 Kas expected/count/variance shift terbuka disembunyikan (NULL) untuk melindungi hitung buta. **Implementasi; BELUM DIUJI.**
- [x] B500-09 Endpoint GET /shifts/{id}/notes: validasi tenant/cabang/pelaku dan shift harus tertutup. **Implementasi; BELUM DIUJI.**
- [x] B500-10 Pagination catatan koreksi append-only beserta pembuat dan waktu server. **Implementasi; BELUM DIUJI.**
- [x] B500-11 Endpoint POST /shifts/{id}/notes: izin shift.view_all, validasi UTF-8/trim/maksimal 1000 karakter. **Implementasi; BELUM DIUJI.**
- [x] B500-12 Idempotency wajib untuk catatan; insert catatan + audit + respons replay satu transaksi, akses diperiksa sebelum replay. **Implementasi; BELUM DIUJI.**
- [x] B500-13 Audit penambahan catatan hanya note_id tanpa body catatan agar isi bebas tidak tersebar ke audit. **Implementasi; BELUM DIUJI.**
- [x] B500-14 Halaman /shift lazy-loaded, pengenal rute aplikasi/preview, dan menu riwayat untuk peran kasir berizin. **Implementasi; BELUM DIUJI.**
- [x] B500-15 UI diisolasi per bisnis/pengguna/cabang, state pemulihan pending per cakupan. **Implementasi; BELUM DIUJI.**
- [x] B500-16 UI filter tanggal/status/terminal termasuk nonaktif dan pagination riwayat shift. **Implementasi; BELUM DIUJI.**
- [x] B500-17 Buka ringkasan shift tertutup dari sejarah meski tab/halaman sebelumnya sudah ditutup, tanpa bocor ringkasan shift terbuka. **Implementasi; BELUM DIUJI.**
- [x] B500-18 Rincian ringkasan uang/payment/refund/void/diskon memakai BigInt dan waktu zona cabang, dukungan cetak. **Implementasi; BELUM DIUJI.**
- [x] B500-19 UI baca catatan dengan pagination dan form append-only khusus pemilik/manajer, peringatan jangan masukkan rahasia/PII. **Implementasi; BELUM DIUJI.**
- [x] B500-20 Pemulihan penambahan catatan memakai snapshot/key asli B290, blok aksi baru saat belum pasti; kontrak 3 endpoint didokumentasikan implemented-not-tested. **Implementasi; BELUM DIUJI.**

Migration 0065 harus diterapkan sebelum membuka /shift. Catatan hanya anotasi; bukan dokumen refund/ubah kas, dan tidak mengubah expected/count/variance shift final. Riwayat penjualan/kas tetap di endpoint lama. Tidak ada fitur edit/hapus catatan.

Backend: shift_history.go, shift.go, migrasi 0065; frontend: ShiftHistoryPage.jsx, App.jsx, AdminLayout.jsx, website.js (hanya route admin). OpenAPI 3 endpoint diperbarui melalui generator sementara; generator bukan suite testing dan tidak mem-build aplikasi runtime.

Batas: terminal selector mengikuti 100 terminal pertama API; cetak hanya halaman catatan yang dimuat. Permission, query SQL, mutasi, JSX, waktu lokal, retry, dan migration belum diverifikasi. Tidak ada perubahan landing/provider luar.

- [x] **DITUNDA: testing B500 BELUM DIJALANKAN/BELUM LULUS.**
