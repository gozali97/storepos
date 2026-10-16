# Pelanggan opsional — CORE-035

- POST /customers: `{name,phone,note}`; nama wajib maksimal 200 karakter, HP opsional maksimal 32 byte, catatan maksimal 200 karakter. Tidak ada NIK/alamat/data kesehatan.
- GET /customers?after=UUID: pelanggan aktif tenant, 100 item dan next_cursor. Izin operasional sale.create.
- DELETE /customers/{id}, If-Match: anonimisasi oleh peran business.manage. Baris tetap ada untuk histori, nama menjadi Pelanggan dihapus, HP/catatan dikosongkan. Versi lama ditolak; anonimisasi ulang tidak membuat salinan PII.
- POST /sales/finalize menerima customer_id opsional. Referensi harus milik tenant dan belum dianonimkan. Validasi/penautan berada dalam transaksi finalisasi; kegagalan tidak menyimpan penjualan atau pergerakan stok.

Sales hanya menyimpan referensi customer_id, bukan salinan nama/HP. Pembaca histori harus join customers untuk menghormati anonimisasi, jangan membuat snapshot PII di struk atau audit. Audit hanya menyimpan ID dan tindakan, bukan isi pelanggan.

Belum: edit/pencarian pelanggan, endpoint detail transaksi dengan nama pelanggan, pilihan pelanggan pada draft, dan UI. Kebijakan retensi/anonimisasi backup dan kesesuaian UU PDP tetap membutuhkan validasi hukum; anonimisasi DB aktif saja bukan penghapusan semua salinan backup.
