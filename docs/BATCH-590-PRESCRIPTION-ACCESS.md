# B590 — referensi resep terbatas

Migrasi 0066 diterapkan lokal untuk metode bayar. Tidak ada migrasi baru resep (memakai kolom snapshot existing).

- [x] Baca referensi terpisah dari detail sale/struk, role Apoteker + permission/module/tenant/branch wajib.
- [x] Audit baca harus berhasil sebelum respons dikirim; audit tanpa teks resep. Header no-store/private, Pragma no-cache, no-referrer.
- [x] UI explicit reveal/hide, hide visibility/beforeprint, tidak menaruh referensi di storage.
- [x] DITUNDA: test/vet/build aplikasi/browser belum dijalankan/belum lulus.

Batas: referensi tersimpan plaintext pada kolom existing, akses DBA/backup bukan dikontrol UI; no-store tidak menjamin data tidak disalin pengguna berizin. Validasi field minimum/regulasi/retensi tetap memerlukan pihak luar. Tidak mengklaim kepatuhan atau fitur pilot siap.
