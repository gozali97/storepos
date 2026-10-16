# Cetak ulang — CORE-022

POST /api/v1/sales/{id}/reprints dengan sesi/CSRF/tenant dan izin sale.create serta akses cabang. Respons 201 `{label:"SALINAN",sale_id,detail_path:"/api/v1/sales/{id}"}`.

Backend menulis receipt.reprint_requested dengan pelaku/cabang/terminal/IP/user-agent dalam audit. Detail snapshot dibaca dari endpoint detail transaksi. Jangan kirim hasil printer sebagai bukti pembayaran atau mengubah status transaksi bila browser/printer gagal.

Event audit berarti permintaan cetak ulang, bukan bukti kertas berhasil dicetak. Agent frontend perlu menghubungkan tombol cetak ulang ke endpoint ini dan mempertahankan label SALINAN. Tidak ada perubahan frontend dalam batch backend ini. Template lengkap snapshot bisnis/header/footer/diskon/SC dan kalibrasi printer fisik masih terbuka.

Setiap POST adalah permintaan audit baru; tidak memakai idempotency key karena tidak mengubah uang/stok. Replay jaringan dapat menghasilkan dua event permintaan, bukan duplikasi penjualan.
