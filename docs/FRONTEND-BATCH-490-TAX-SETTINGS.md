# B490 — 20 implementasi UI pajak & penugasan

**Testing/vet/build aplikasi/browser ditunda. [x] hanya implementasi, bukan lulus atau siap produksi.**

- [x] B490-01 Halaman /pajak lazy-loaded, rute aplikasi/preview, menu dan tautan dari Pengaturan. **Implementasi; BELUM DIUJI.**
- [x] B490-02 Pembatasan UI hanya Pemilik dan reset workspace per bisnis/pengguna. **Implementasi; BELUM DIUJI.**
- [x] B490-03 Daftar aturan scoped aktif dan arsip dari backend dengan status/rate/mode. **Implementasi; BELUM DIUJI.**
- [x] B490-04 Pagination aturan cursor halaman pertama/berikutnya. **Implementasi; BELUM DIUJI.**
- [x] B490-05 Muat detail aturan terbaru sebelum membuka editor, bukan memakai snapshot list lama. **Implementasi; BELUM DIUJI.**
- [x] B490-06 Form buat aturan scoped dengan nama maksimal 100 karakter. **Implementasi; BELUM DIUJI.**
- [x] B490-07 Konversi persen tepat ke basis point string/integer terbatas 0–10000 tanpa perhitungan float tarif. **Implementasi; BELUM DIUJI.**
- [x] B490-08 Pilihan inclusive/exclusive sesuai endpoint dan validasi helper taxRuleBody. **Implementasi; BELUM DIUJI.**
- [x] B490-09 Edit aturan memakai If-Match versi aturan dan reset form menurut ID/versi. **Implementasi; BELUM DIUJI.**
- [x] B490-10 Konflik versi editor mengunci submit sampai detail terbaru dimuat ulang. **Implementasi; BELUM DIUJI.**
- [x] B490-11 Konfirmasi arsip inline tanpa window.confirm, dengan penjelasan aturan masih ditugaskan ditolak backend. **Implementasi; BELUM DIUJI.**
- [x] B490-12 Pulihkan aturan arsip memakai versi dan reload daftar. **Implementasi; BELUM DIUJI.**
- [x] B490-13 Pencarian produk/SKU untuk memilih target pajak, varian satu produk dideduplikasi. **Implementasi; BELUM DIUJI.**
- [x] B490-14 Pemilih kategori aktif sebagai target penugasan pajak. **Implementasi; BELUM DIUJI.**
- [x] B490-15 Muat penugasan target dan versi master produk/kategori dari server. **Implementasi; BELUM DIUJI.**
- [x] B490-16 Tampilkan nama aturan target saat berada di halaman aturan lain melalui lookup detail. **Implementasi; BELUM DIUJI.**
- [x] B490-17 Simpan penugasan pajak produk/kategori dengan If-Match, lepas memakai null untuk inheritance. **Implementasi; BELUM DIUJI.**
- [x] B490-18 Pilihan assignment hanya aturan aktif; aturan arsip saat ini ditandai dan tidak dipilih sebagai penugasan baru. **Implementasi; BELUM DIUJI.**
- [x] B490-19 Form pajak default memuat nilai server, mengubah nama/tarif/mode, mendukung tarif 0%, dan refresh setelah simpan. **Implementasi; BELUM DIUJI.**
- [x] B490-20 Penjelasan prioritas produk→kategori→default serta snapshot transaksi lama; state loading/error/empty/busy, konflik assignment wajib reload dan tutup editor tersedia. **Implementasi; BELUM DIUJI.**

Berkas utama: TaxSettingsPage.jsx, taxes.js, App.jsx, AdminLayout.jsx, SettingsPage.jsx, website.js (hanya pengenal rute admin; tidak mengubah landing). Endpoint backend sudah ada, tanpa migrasi/runtime backend baru.

Batas: pilihan aturan assignment mengikuti halaman aturan yang dimuat (100 per halaman); pencarian produk dibatasi endpoint 30 hasil; kategori aktif tidak berhalaman. Konflik tetap divalidasi server. Pajak default memakai endpoint lama tanpa versi; schema/JSX/runtime belum diuji, regulasi pajak tidak divalidasi.

- [x] **DITUNDA — testing B490 BELUM DIJALANKAN/BELUM LULUS.**
