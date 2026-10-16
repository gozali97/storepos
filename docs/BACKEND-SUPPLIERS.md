# Supplier — CORE-033

- `POST /api/v1/suppliers`: `{name,contact,address,note,archived}`.
- `GET /api/v1/suppliers?after=UUID`: daftar termasuk arsip, maksimal 100 item dan next_cursor.
- `PUT /api/v1/suppliers/{id}` dengan If-Match: ganti seluruh field; `archived:true` mengarsipkan supplier.
- `DELETE /api/v1/suppliers/{id}` dengan If-Match: hanya jika belum dipakai penerimaan. Jika sudah dipakai, HTTP 409 meminta arsip.

Endpoint membutuhkan izin supplier.manage, tenant, sesi, dan CSRF untuk mutasi. Nama/kontak maksimal 200 karakter, alamat/catatan 1000 karakter. Versi edit lama ditolak. Catatan audit tidak mencatat isi kontak/alamat.

`POST /api/v1/goods-receipts` menerima `supplier_id` opsional. Supplier harus milik tenant dan aktif. Supplier yang diarsipkan tetap direferensikan dokumen lama; tidak dapat dipakai dokumen baru. Lock mencegah delete/archive berlomba dengan pembuatan penerimaan.

Supplier apotek yang wajib belum diaktifkan karena jalur penerimaan apotek masih diblokir sampai batch/ED tersedia.
