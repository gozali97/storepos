# StorePOS App

Aplikasi kasir & back-office React dengan layout admin dan login Soft UI bertema biru. Keputusan frontend ada di `../docs/adr/0004-frontend-soft-ui.md`; status tiap kebutuhan ada di `../TASKS.md` §1.8 dan roadmap Fase 2.

Jalankan backend dari folder `backend` dengan `air`, lalu dari folder `app`:

```bash
npm run dev
```

Buka `http://localhost:5173/` untuk landing publik, `/masuk` untuk login, dan `/dashboard` untuk admin setelah masuk. Landing tetap dapat dibaca ketika backend belum hidup karena tidak menjalankan pemeriksaan sesi. Proxy API mengarah ke `http://127.0.0.1:8090`; atur `STOREPOS_API_URL` di `app/.env` bila alamat backend berbeda (contoh di `.env.example`). Restart Vite setelah perubahan environment. Akun, bisnis, cabang, dan terminal berasal dari backend. Tidak ada akun demo yang di-hardcode.

`npm run build` membangun aplikasi dan landing menjadi satu folder `app/dist`: `index.html` untuk landing, `admin.html` untuk entry React, serta seluruh aset/halaman publik. `npm run preview` melayani kedua entry sesuai alamat. Untuk hosting produksi, arahkan route aplikasi seperti `/masuk`, `/dashboard`, `/kasir`, dan `/pesanan/:id` ke `admin.html`; root dan halaman publik dilayani sebagai berkas statis. Berkas `_redirects` disediakan untuk host yang mendukung fallback Netlify. Contoh Nginx ada di `../landing/README.md`.

Halaman tersedia: dashboard, kasir, katalog, persediaan, laporan, supplier, pelanggan, pengeluaran, pengaturan, anggota dan log aktivitas. Navigasi serta data dibatasi menurut akses akun. Layar kasir mendukung draft per terminal, tahan/lanjutkan transaksi, buka shift, pembayaran tunai/nontunai/split, serta retry finalisasi dengan kunci idempoten yang sama. Halaman persediaan mencakup penerimaan beberapa barang, penyesuaian dengan retry aman, kartu stok, stok minimum/daftar pengadaan, dan opname bertahap.

Sudah dibuat:

- `src/money.js` — utilitas uang/kuantitas berbasis `BigInt` (PRD §13.7).
- `src/pricing/engine.js` — port mesin harga backend (PRD §13.8). Diuji dengan vektor bersama di `backend/internal/pricing/testdata/vectors.json`.
- `src/pricing/engine.test.js` — uji dengan `node --test`.

Jalankan uji vektor:

```bash
npm test
npm run build
```

Tes render memakai fixture khusus pengujian, bukan data produksi. Tes itu tidak menggantikan pemeriksaan browser/mobile atau login terhadap backend nyata.

Pengaturan Pemilik mendukung kebijakan diskon/approval/pembulatan, tambah/edit cabang dan teks struk, serta tambah/nonaktifkan terminal dengan versi server. Pengeluaran mendukung kas shift terminal aktif dan approval PIN; Kasir hanya dapat memakai kas shift.

Laporan performa dapat dikelompokkan per produk atau kategori. Ekspor latar belakang tersedia melalui tombol buat antrean, periksa status, dan unduh hasil CSV setelah worker selesai.

Halaman Anggota tersedia untuk Pemilik/Manajer: akses peran/cabang/status dengan versi server serta undangan manual. Penerima dapat memasukkan token lewat Pengaturan. Katalog mendukung pengelolaan satuan tambahan, faktor konversi, harga jual dan barcode satuan. Kasir mendukung pemilihan satuan jual dan scan barcode satuan, subtotal berdasarkan harga satuan, serta opsi pencegahan scan ganda dan bunyi sukses/gagal.

Kasir menyediakan keypad layar untuk kuantitas dan pembayaran (termasuk split), serta indikator input scanner cepat yang diakhiri Enter. Kalibrasi scanner HID dan pemeriksaan pada tablet/perangkat nyata masih perlu dilakukan.

Halaman Riwayat transaksi mendukung pencarian nomor struk/periode cabang, detail snapshot pembayaran/barang, void dengan PIN, dan retur qty/kondisi per barang. Koreksi memakai retry idempoten. Retur split pembayaran dan apotek belum didukung. Pelanggan opsional dapat dipilih saat checkout; batas hari retur dapat diatur oleh Pemilik.

Struk digital tersedia dari checkout dan Riwayat transaksi: buat tautan/QR, buka `/receipt#token` tanpa login, dan cabut tautan sebagai Pemilik. Halaman publik mengirim token di body POST. Template cetak 58/80 mm tersedia; pilih ukuran kertas yang sama di dialog printer. Cetak Riwayat berlabel SALINAN, audit cetak ulang dipanggil sebelum dialog cetak dibuka; kalibrasi printer fisik masih backlog.

Katalog menyediakan produk multivarian dengan maksimal 3 atribut/100 kombinasi, pratinjau dari backend, dan SKU/harga editable sebelum penyimpanan. Header admin memperingatkan selisih jam lebih dari lima menit berdasarkan header Date API; server/proxy perlu menyertakan header tersebut.

Kasir memeriksa harga dan ketersediaan produk/satuan sebelum membuka checkout dan sebelum finalisasi. Perubahan harga memperbarui subtotal dan meminta tinjau ulang; produk/satuan tidak tersedia menghalangi pembayaran. Harga final mengikuti perhitungan server.

Mode offline kasir menyimpan katalog/satuan dan keranjang pada localStorage per akun/bisnis/cabang/terminal. Gunakan tombol Keranjang offline atau putuskan koneksi setelah sesi/katalog dimuat. Bayar diblokir; saat online, sinkronkan draft lalu periksa harga sebelum finalisasi. Konflik versi mempertahankan keranjang lokal. Startup/login tanpa koneksi dan finalisasi offline belum tersedia.
