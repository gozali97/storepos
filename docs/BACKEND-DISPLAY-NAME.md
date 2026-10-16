# Nama tampilan dan snapshot kasir

GET /api/v1/me/profile mengembalikan display_name dan version. PUT endpoint sama dengan If-Match, `{display_name:"Kasir"}` mengubah nama akun sendiri. Sesi aktif/CSRF diperlukan, tidak memerlukan tenant; nama adalah profil global pengguna multi-bisnis. Maksimal 200 karakter, tidak kosong/karakter kontrol. Versi stale 409.

Audit user.profile_changed hanya ID/versi, tidak salinan nama. Finalisasi baru menyimpan cashier_name di receipt_header_snapshot, memakai display_name atau NULL bila belum diisi. Tidak memakai email/nomor HP sebagai nama kasir. Detail transaksi/public receipt menerima snapshot kasir tersebut; perubahan profil berikutnya tidak mengganti nama histori.

Migrasi 0031 tidak mengisi nama dari email atau merekonstruksi histori. Nama kasir merupakan data pribadi yang menjadi snapshot struk publik; operator perlu menjelaskan kebijakan nama profesional/pseudonim dan retensi. Anonimisasi pelanggan berbeda dari penghapusan nama kasir histori; kebijakan privasi/legal masih terbuka.

Frontend tidak diubah. /me tetap mempertahankan kontrak lama; baca nama via /me/profile. Pembaruan email/HP/password dan penghapusan akun bukan bagian endpoint ini.
