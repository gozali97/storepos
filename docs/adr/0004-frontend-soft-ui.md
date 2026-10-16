# ADR 0004: frontend React dan tampilan Soft UI biru

Status: diterapkan untuk layout admin dan login, 13 Oktober 2026.

## Keputusan

Pertahankan React 18, JavaScript ESM, Vite, dan React Router yang sudah terpasang. Gunakan Context untuk sesi dan ruang kerja, state lokal untuk formulir, dan lazy imports per halaman. Tidak menambah framework UI, state library atau dependensi runtime.

TypeScript tetap merupakan opsi migrasi tersendiri; redesign ini tidak memerlukan penulisan ulang proyek. Nilai uang diproses dengan BigInt, dikirim sebagai string sen, dan diuji memakai vektor bersama backend.

Sesuai arahan pengguna, aplikasi mengikuti [Soft UI Dashboard Tailwind](https://github.com/creativetimofficial/soft-ui-dashboard-tailwind) dengan aksen biru: latar abu terang, sidebar, navigasi aktif putih dengan ikon biru, panel putih, radius lembut, bayangan dan tombol gradasi biru. Login memiliki form kiri dan cover biru kanan yang menghilang pada layar sempit. Implementasi tetap CSS lokal, menyesuaikan markup React dan konten StorePOS. Tidak menyalin script analytics, link pemasaran template atau data contoh dashboard ke aplikasi.

Warna aplikasi menggantikan arahan Kertas/Tinta/Kunyit pada frontend app. Landing page merupakan bundle dan pekerjaan terpisah. Anti Slop dinonaktifkan untuk pekerjaan ini atas permintaan pengguna.

## Kontrak dan batasan

Sesi tetap cookie HttpOnly dan CSRF. Query menggunakan klien API bersama, konteks bisnis, AbortSignal, dan membuang hasil yang tidak lagi sesuai ruang kerja. Endpoint `GET /api/v1/workspace` menyediakan cabang/terminal aktif hanya dalam tenant dan penugasan pengguna. Pemilihan ruang kerja disimpan per bisnis dan diverifikasi kembali terhadap respons server.

Navigasi mengikuti peran untuk kenyamanan. Backend tetap menjadi penentu izin; menu tersembunyi tidak dianggap sebagai pengamanan data. Dashboard memakai data laporan nyata dan mempertahankan nilai NULL untuk histori yang belum lengkap.

Pengeluaran tidak menggunakan floating point. Request yang belum pasti menggunakan payload dan idempotency key yang sama, disimpan sementara per pengguna/bisnis dalam sessionStorage; tidak ada PIN atau kata sandi di penyimpanan itu.

Code splitting modul restoran/apotek belum dapat dinyatakan selesai sebelum halaman modul tersebut tersedia. Validasi browser/perangkat dan integrasi PostgreSQL harus dijalankan pada lingkungan yang mengizinkan server dan browser.
