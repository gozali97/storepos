# Kontak dan routing snapshot restoran

POST /restaurant/orders menerima contact_name/contact_phone/queue_label opsional, dengan aturan tipe sama seperti header edit: DINE_IN tanpa kontak/antrean, TAKEAWAY nama/antrean tanpa HP, DELIVERY_MANUAL boleh nama/HP/antrean/kanal/ongkir. Ongkir maksimal Rp100 juta. Resuming meja existing tidak mengganti kontak/header lewat POST.

DELETE /restaurant/orders/{id}/contact, If-Match, Pemilik/business.manage: kontak nama/HP/antrean dikosongkan, order_version naik, audit hanya ID. Bekerja pada order open/closed, tanpa menghapus transaksi. Note/channel tidak dibersihkan karena bukan field kontak formal; operator jangan memasukkan PII ke note/channel. Backup dan log eksternal perlu kebijakan penghapusan/retensi legal tersendiri.

Migrasi 0050 menambah category_id snapshot pada restaurant_order_lines. Tambah item menyimpan kategori produk saat itu; send memakai snapshot, bukan kategori master terbaru. Histori diisi kategori saat migrasi dengan category_snapshot_legacy=true agar tidak diklaim historis terverifikasi. Snapshot kategori null tetap memblokir send tanpa mapping; tidak diganti diam-diam.

Harga/modifier/kategori snapshot tidak berarti checkout selesai. Modul order belum memiliki pembayaran final dan batch stok. Tanpa fungsi tes baru, build/vet/suite lama lulus. Frontend tidak diubah.
