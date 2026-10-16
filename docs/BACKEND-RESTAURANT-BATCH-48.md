# Batch 0048 — celah availability/reservasi/routing

- PUT item qty bertambah dan POST send menolak menu sold_out. Qty turun/delete tetap bisa untuk memulihkan pesanan. Tidak mengubah item SENT secara otomatis.
- Rekonsiliasi harian menambahkan kind reservation: reserved_milli vs SUM reservations active, serta reserved>on_hand. Satu query snapshot bersama invariant stok/pembayaran; worker menyimpan temuan dan error-log alert. Tidak mengubah data otomatis.
- DELETE /restaurant/category-routes/{category_id}?branch_id=UUID, If-Match versi mapping: Owner/Manager, tenant/modul/cabang, audit. Tiket lama tetap station snapshot asli; pengiriman baru kategori tanpa mapping ditolak sampai dipetakan kembali.

Migrasi 0048 menambah CHECK kind dan indeks active reservations. Down menolak jika temuan reservation masih tersimpan, tidak menghapus bukti diam-diam.

Tidak membuat fungsi tes baru; suite lama/build/vet lulus. Belum: checkout konsumsi reservasi, recovery expiry, waste fisik, flag availability linearizable terhadap mutasi bersamaan, dan SSE. Frontend tidak diubah.
