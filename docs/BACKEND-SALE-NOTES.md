# Snapshot catatan — CORE-014 / §13.6

POST /sales/finalize menerima note di header dan pada setiap lines[i].note. Maksimal 200 karakter Unicode valid UTF-8, default string kosong untuk kompatibilitas request lama. Jika draft_id digunakan, note header/baris harus sama dengan cart draft tersimpan; perubahan catatan di request pembayaran ditolak 409 sebelum efek transaksi.

Catatan disimpan di sales.note dan sale_lines.note dalam transaksi finalisasi. GET /sales dan GET /sales/{id} mengembalikan catatan untuk pengguna terautentikasi/berizin. Audit finalisasi tidak menyalin isi catatan, hanya metadata nominal/ID. Public receipt belum menampilkan catatan agar teks yang berpotensi sensitif tidak dipublikasikan otomatis.

Migrasi 0030 memakai batas char_length DB sebagai lapisan kedua. Histori lama memiliki string kosong, bukan catatan historis yang direkonstruksi. Frontend tidak diubah; agent frontend perlu meneruskan note saat finalisasi draft. Nama kasir dan modifier snapshot masih terpisah/belum tersedia.
