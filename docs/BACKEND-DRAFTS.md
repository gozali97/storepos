# API keranjang draft

- `POST /api/v1/drafts`: `{branch_id, register_id, cart}` membuat satu draft aktif per terminal.
- `GET /api/v1/drafts?register_id=...`: mengambil draft aktif dan tertahan; akses dibatasi tenant/cabang.
- `PUT /api/v1/drafts/{id}` dengan `If-Match`: `{status, cart}` mengganti keranjang dan status `active`, `held`, atau `cancelled`. Kirim keranjang lengkap; catatan maksimal 200 karakter.
- `POST /api/v1/sales/finalize`: tambahkan `draft_id` dan `draft_version` beserta baris, diskon dan pembayaran. Baris/diskon harus sama dengan draft. Penjualan dan perubahan draft menjadi `completed` berada dalam transaksi yang sama. Idempotency replay tidak menjual ulang draft.

Status selesai/dibatalkan tidak dapat dibuka kembali. Maksimal satu draft aktif dan sepuluh tertahan per terminal. Draft tidak menyimpan PIN atasan atau pembayaran.

## Pekerjaan tersisa CORE-014

Penyimpanan draft belum menghitung pratinjau total, memvalidasi kebijakan pecahan varian, atau menggabungkan scan berulang; finalisasi tetap memakai harga server. Catatan tersimpan di draft, belum menjadi snapshot catatan struk. Penghapusan draft lama belum tersedia. Karena itu CORE-014 secara keseluruhan belum selesai.
