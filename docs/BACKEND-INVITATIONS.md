# Undangan — CORE-004

POST /invitations: `{email,role,branch_ids}` untuk Owner/Manager. Manager tidak bisa mengundang Owner/Manager atau cabang di luar penugasannya. Token acak 256 bit dikembalikan sekali, disimpan hanya SHA-256, expires_at tujuh hari. Cache-Control no-store, delivery=manual; jangan memasukkan token ke log/analytics/URL publik.

POST /invitations/accept: `{token}` dengan sesi pengguna dan CSRF, tanpa memerlukan keanggotaan tenant tujuan. Email akun aktif harus sama dengan email undangan dan email_verified_at wajib terisi; akun belum terverifikasi ditolak. Lock tenant/undangan mencegah penerimaan dua kali. Izin pengundang, status cabang dan cakupan Manager diperiksa ulang saat diterima. Keanggotaan yang sudah ada tidak ditimpa atau diaktifkan ulang; kelola lewat endpoint membership.

DELETE /invitations/{id}: revoke undangan yang belum diterima. Manager hanya bisa mencabut undangannya sendiri; Owner seluruh undangan bisnis. Tenant lain tidak dapat mengakses. Penerimaan undangan revoked/expired/accepted ditolak.

Audit created/accepted/revoked menyimpan ID/peran/cabang, tidak kontak/token. Token index unik ditambahkan oleh migrasi 0024.

Belum: pengiriman email/WhatsApp, nomor HP, daftar undangan, registrasi akun penerima baru dan verifikasi email/OTP. Penerimaan membutuhkan akun aktif yang sudah terverifikasi. Alur verifikasi publik belum tersedia, jadi akun email_verified_at NULL tidak dapat menerima undangan; jangan mengisi flag ini tanpa verifikasi nyata. Frontend tidak diubah.
