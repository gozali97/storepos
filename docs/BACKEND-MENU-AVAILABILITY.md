# Ketersediaan menu — RST-013

PUT /restaurant/menu/{variant_id}/availability, If-Match (0 baru/versi berikutnya), {branch_id,sold_out:true|false}, Owner/Manager. Varian/cabang aktif, tenant sama, versi/audit. GET /restaurant/menu/availability?branch_id=UUID&after=UUID untuk pengguna order/sale.create mengembalikan flag manual dan on_hand/reserved/available. Default sold_out false/version 0 bila belum diatur.

POST baris pesanan menolak menu ditandai Habis. Flag tidak menghapus item yang sudah masuk atau dikirim; reservasi kirim tetap memvalidasi saldo. Recheck sold_out kini berlaku saat qty NOT_SENT ditambah dan saat kirim ke dapur; menurunkan qty/hapus tetap diperbolehkan. Item yang sudah SENT tidak dibatalkan otomatis oleh flag. Pemeriksaan membaca flag saat mutasi, belum mengunci flag terhadap perubahan simultan sehingga request yang sudah berjalan dapat selesai sebelum flag baru efektif. Flag diatur per cabang, bukan global semua toko.

GET /reports/stock dan /stock/reorder kini mengembalikan reserved_milli/available_milli. Minimum dihitung memakai available=on_hand-reserved; nilai stok tetap berdasarkan on_hand. CSV stok menambahkan kedua kolom di akhir agar indeks kolom lama tidak berubah. RETURN_DAMAGED tetap bukan delta saldo jual.

Migrasi 0047. Tanpa fungsi testing baru; build/vet/suite lama lulus. Frontend tidak diubah. Menu tanpa track_stock tidak otomatis habis karena saldo nol; pakai flag manual. Checkout/konversi reservasi belum tersedia. Rekonsiliasi reserved vs reservasi aktif/lebih besar on_hand kini masuk worker harian sebagai kind=reservation (migrasi 0048), tanpa perbaikan otomatis.
