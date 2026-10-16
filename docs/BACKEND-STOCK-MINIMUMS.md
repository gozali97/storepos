# Stok minimum — RTL-006

`PUT /api/v1/stock/minimums/{variant_id}` dengan `{branch_id, minimum_qty_milli}` dan If-Match. Gunakan If-Match: 0 untuk buat, versi saat ini untuk edit. Minimal 5 unit = `"5000"`. Hanya izin product.manage dengan akses cabang. Produk harus melacak stok; minimum tidak negatif.

`GET /api/v1/stock/reorder?branch_id=UUID&after=UUID` mengembalikan varian aktif dengan saldo <= minimum. Saldo belum ada dianggap nol. Maksimal 100 item dengan next_cursor. Angka qty/minimum berupa string milli-unit. Supplier berasal dari penerimaan posted terakhir yang memiliki supplier pada varian/cabang terkait; tanpa histori, supplier null. Supplier arsip tetap terlihat sebagai histori pengadaan.

Daftar dibaca dari saldo terkini; tidak ada cache peringatan yang harus disinkronkan. Versi lama ditolak 409, akses lintas tenant/cabang ditolak. UI in-app, notifikasi eksternal dan uji beban katalog besar belum tersedia.
