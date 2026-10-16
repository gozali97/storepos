# Kuantitas pecahan — RTL-003

Saat `POST /api/v1/products`, setiap varian dapat menyertakan `allow_fractional:true`. Default false. Pencarian katalog dan respons produk mengembalikan flag tersebut.

`qty_milli` tetap string integer: `"375"` = 0,375 satuan dasar. Presisi lebih dari tiga desimal tidak diterima parser. Varian tanpa izin pecahan hanya menerima kelipatan 1000 pada draft/finalisasi. Varian harus aktif dan milik tenant saat draft disimpan.

Penjualan menyimpan snapshot allow_fractional di sale_lines. Retur mengikuti snapshot, bukan pengaturan varian terkini. Migrasi menandai histori sale_lines yang sudah berkualitas pecahan agar retur lama tetap didukung.

Contoh teruji: harga Rp14.000/kg (`price_cents:"1400000"`), kuantitas 0,375 (`qty_milli:"375"`), total Rp5.250. Pembayaran tunai tetap mengikuti pembulatan tunai bisnis yang sementara Rp100; contoh harga tepat menggunakan QRIS manual.

Belum: endpoint edit kebijakan pecahan setelah produk dibuat, kebijakan satuan beli/jual tambahan, pembatasan pecahan apotek, dan integrasi timbangan. Migrasi wajib diterapkan sebelum menjalankan binary baru.
