# Pencarian/detail penjualan — CORE-028

GET /api/v1/sales?branch_id=UUID&receipt_number=... atau from/to=YYYY-MM-DD. Nomor struk eksak atau periode wajib, maksimal 366 hari zona lokal cabang. before=UUID memakai pasangan completed_at/id, cursor wajib berasal dari cakupan/filter sama. Maksimal 100 transaksi dengan next_cursor. Transaksi voided/retur tetap tampil untuk koreksi, bukan laporan penjualan neto.

GET /api/v1/sales/{id}: sale header, snapshot lines dan payments. Setiap baris memuat sale_line_id, qty satuan jual/dasar, faktor, harga asli, returned_qty_milli dan remaining_qty_milli. Gunakan ID baris dan qty sisa untuk endpoint retur. Saat voided, status sale harus dipatuhi; remaining_qty tidak berarti voided boleh diretur. Header/baris/pembayaran dibaca dengan lock share transaksi asal agar void/return tidak menghasilkan snapshot campuran.

Izin sale.return baca (termasuk kasir dengan grant O) dan akses cabang wajib. Ini hanya membaca; mutasi retur tetap memerlukan approval sesuai peran. cost_cents hanya dikirim jika product.view_cost allowed. Nama pelanggan diambil dari referensi current customers, jadi anonimisasi berlaku tanpa salinan HP. Respons no-store. HPP/harga/qty transaksi tidak dibaca dari katalog terkini.

Belum: URL/QR struk digital, filter pencarian bebas/nomor parsial, detail refundable amount preview, rincian header tax-rule/discount/SC dan snapshot catatan lengkap. API frontend tidak diubah.
