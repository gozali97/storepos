# Struk digital — CORE-023

POST /api/v1/sales/{id}/receipt-links (sale.create + akses cabang, sesi, CSRF) menghasilkan `{token,receipt_path:"/receipt#spr_..."}`. Token 256 bit disimpan hanya hash SHA-256. Token dikeluarkan sekali, tidak ditulis audit. Fragment URL tidak dikirim pada HTTP sehingga menghindari token dalam access-log path/query.

Frontend agent terpisah perlu membuat /receipt yang membaca fragment dan POST /api/v1/public/receipt `{token}`. Endpoint publik tanpa sesi mengembalikan snapshot nama barang/varian, satuan, kuantitas, harga/total, nomor/status/tanggal transaksi, pajak/pembulatan, nama pelanggan bila ada. Tidak ada HP/catatan pelanggan, ID tenant/user, harga beli/HPP, PIN/password atau token di respons receipt. Nama pelanggan diambil dari referensi untuk menghormati anonimisasi.

DELETE /api/v1/sales/{id}/receipt-links (business.manage/Pemilik) mencabut semua tautan transaksi secara atomik dan audit. Token salah atau dicabut 404, UI menampilkan Struk tidak tersedia. Link baru dapat dibuat sesudah revoke; itu keputusan operasi baru, bukan membuka token lama.

No-store/no-referrer. Halaman/QR tidak dibuat pada pengerjaan backend. Header bisnis/cabang/alamat/zona/header/footer sekarang disnapshot saat finalisasi (migrasi 0029). Public receipt mengembalikan header_snapshot, subtotal_gross_cents, discount_cents dan service_charge_cents. Histori lama tetap NULL dengan header_snapshot_missing=true; tidak mengambil pengaturan master terbaru untuk mengklaim histori asli.

Belum: snapshot nama kasir, rincian diskon/SC per baris, pengiriman WhatsApp/email/SMS, token per transaksi otomatis saat finalisasi, masa retensi link. Rate limit endpoint publik kini 60/menit/IP peer via PostgreSQL lintas instance dengan Retry-After 60 detik; cleanup entri >1 hari oleh worker. Window tetap per menit (bukan sliding), proxy headers tidak dipercaya sehingga deployment reverse proxy perlu konfigurasi peer tepercaya sebelum memakai IP forwarded. Bearer token memberi akses nama pelanggan; jangan masukkan ke analytics, log request body, atau penyedia QR eksternal.
