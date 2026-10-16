# Landing StorePOS

Halaman marketing statis terpisah dari bundle React aplikasi. Desain mengikuti susunan Bigspring dari gambar referensi pengguna: hero terpusat, enam fitur, konten bergantian, alur kerja, konsultasi, dan footer. Warna mengikuti brand StorePOS biru. Logo menggunakan ketiga aset pengguna dari `app/public/images`, disalin ke `landing/public/images`.

## Menjalankan bersama aplikasi (utama)

```sh
cd app
npm run dev
```

Buka `http://localhost:5173/`: landing publik. Tombol Masuk menuju `/masuk` pada origin yang sama. Sesudah login, aplikasi membuka `/dashboard`; halaman kasir dan operasional tetap memakai alamat sebelumnya. Akses landing tidak membutuhkan sesi atau backend yang aktif.

`npm run build` dari folder `app` menghasilkan website lengkap dalam `app/dist`. `index.html` adalah landing dan `admin.html` adalah entry aplikasi. `npm run preview` dari `app` memilih entry sesuai route. Source landing serta bundle JavaScript tetap terpisah dari bundle React.

Untuk Nginx, contoh routing di server frontend:

```nginx
root /path/to/storepos/app/dist;
location /api/ {
    proxy_pass http://127.0.0.1:8090;
}
location / {
    try_files $uri $uri/ /admin.html;
}
```

Root memakai `index.html` statis, sedangkan route aplikasi tanpa berkas fisik memakai `admin.html`. Hasil build juga mencakup `_redirects` untuk fallback pada hosting yang mendukung aturan Netlify.

## Menjalankan landing terpisah (opsional)

Dependensi build memakai instalasi workspace `app/node_modules`; tidak perlu menambahkan framework atau dependensi landing.

```sh
# Jika dependensi workspace belum terpasang:
cd app
npm ci
cd ../landing
cp .env.example .env
npm run dev
```

Buka `http://localhost:5174`. Aplikasi kasir tetap di `http://localhost:5173/masuk`.

```sh
make landing-dev
make landing-build
make landing-test
```

Build menghasilkan `landing/dist`: HTML, CSS, JavaScript interaksi, aset brand, dan pratinjau komponen aplikasi. Konten utama, FAQ, seluruh jenis usaha, dan langkah demo tersedia tanpa JavaScript. JavaScript meningkatkan menu, tabs/stepper, form, dan animasi. Form sengaja dinonaktifkan jika JavaScript tidak tersedia.

## Konfigurasi & deployment

- `STOREPOS_API_URL`: tujuan proxy `/api` saat development; default `http://127.0.0.1:8090`.
- `STOREPOS_APP_URL`: URL halaman login pada mode landing terpisah; default `http://localhost:5173/masuk`. Mode terpadu memakai `/masuk` pada origin yang sama.
- `STOREPOS_SITE_URL`: domain landing final, tanpa subpath. Jika diisi, build mengeluarkan canonical, Open Graph URL/image, sitemap, dan robots. Jika kosong, halaman tidak diindeks. Kebijakan/ketentuan pengembangan tetap `noindex`.

Saat hosting statis, arahkan `/api/v1/leads` ke backend melalui reverse proxy pada origin yang sama. Proxy development Vite tidak ikut dalam output statis. `ClientIP` backend memakai peer TCP langsung; konfigurasi proxy tepercaya perlu ditetapkan sebelum menerapkan batas per IP pengunjung di produksi.

Sebelum mencoba submit, terapkan migrasi `0060_landing_leads` dan jalankan backend:

```sh
make migrate-up
make backend-run
```

Migrasi membuat `platform.leads`, `platform.lead_requests`, dan `platform.lead_limits`. Endpoint publik `POST /api/v1/leads` menyimpan data/versi persetujuan dan membuat job `lead.notification` dalam transaksi yang sama. Idempotency-Key berupa UUID disimpan juga untuk pengiriman yang didedup; retry dengan kunci sama tetap aman setelah jendela dedup selesai. Dedup telepon + jenis usaha 10 menit, batas 5 pengiriman berkunci baru per peer per jam. Honeypot, batas body, validasi, dan waktu isi minimum tersedia. Pengiriman notifikasi belum aktif karena penyedia TBK-10 belum dipilih. Job hanya berisi ID lead, tanpa data pribadi.

## Interaksi & visual

- Menu mobile memakai dialog native untuk fokus modal dan Escape; fokus kembali ke tombol menu atau bagian tujuan.
- Tab usaha/stepper memakai ARIA dan mendukung Arrow Left/Right, Home, End, serta klik. CTA jenis usaha mengisi pilihan formulir.
- Animasi scroll memakai IntersectionObserver, muncul ulang ketika bagian keluar lalu masuk viewport. Tidak ada listener scroll untuk setiap elemen. `prefers-reduced-motion` mematikan gerak. Tanpa JS, semua konten tetap terlihat.
- Pratinjau dashboard, stok, dan laporan diekspor langsung dari komponen React aplikasi dengan data demo saat build. Tidak mengunduh bundle React di browser landing. Ini pratinjau HTML asli, belum tangkapan layar raster build live untuk LP-002/006.
- Contoh keranjang total Rp175.000, bayar Rp200.000, kembalian Rp25.000. Kas shift akhir mengikuti PRD: Rp200.000 + Rp1.000.000 − Rp50.000 = Rp1.150.000, dihitung Rp1.140.000, selisih −Rp10.000. Batch apotek contoh lama ditandai kedaluwarsa.

## Analytics & pekerjaan tersisa

Event `storepos:analytics` hanya berjalan lokal; tidak ada pengiriman ke penyedia. Filter payload memiliki daftar properti yang diizinkan dan tes untuk menolak field pribadi. Integrasi penyedia dan kebijakan izin analytics menunggu keputusan.

Sebelum rilis publik: validasi dokumen privasi/ketentuan (identitas pengelola, kanal privasi, retensi), pilih penyedia notifikasi/analytics, isi domain dan URL aplikasi, verifikasi endpoint pada PostgreSQL, lakukan pengujian browser/keyboard/mobile, tangkapan layar live, serta Lighthouse/CLS. Halaman perangkat menyatakan status belum diuji tanpa mengarang model atau tanggal hasil uji.

Verifikasi sesi pengerjaan: build landing berhasil dan tes validasi/analytics serta unit validasi backend lulus. Pengujian integrasi PostgreSQL dilewati karena socket dibatasi lingkungan. Chrome lokal juga gagal membuat socket, sehingga audit visual browser dan Lighthouse belum dilakukan.
