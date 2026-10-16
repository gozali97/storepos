# Reservasi restoran — §13.4 / RST-013

Kirim dapur kini mencadangkan item track_stock pada inventory_balances.reserved_milli, tanpa mengubah qty_milli atau membuat pergerakan SALE. Ketersediaan = qty_milli - reserved_milli. Varian dikunci berurutan untuk reservasi banyak item. Reservasi per line tersimpan, qty dari snapshot item.

Pembatalan line melepaskan reservasi aktif tepat sekali dalam transaksi yang sama dengan perubahan/audit pembatalan. Menu tanpa track_stock tidak memiliki reservasi. Perubahan master track_stock sesudah pengiriman tidak menghilangkan kewajiban melepaskan reservasi historis.

Retail finalize memvalidasi stok tersedia setelah reserved. Apply movement keluar (termasuk penyesuaian/opname) tidak dapat menurunkan on-hand di bawah reserved. Stok RESTOCK/RECEIPT masuk tetap boleh. Trigger CHECK menjamin reserved tidak negatif; release memeriksa konsistensi sebelum mengurangi.

Migrasi 0046. Down menolak reservasi aktif agar tidak menghilangkan stok yang masih dicadangkan. Tidak membuat fungsi testing baru; suite lama/build/vet lulus, alur reservasi restoran belum diuji khusus.

Belum: konversi reservasi menjadi SALE saat checkout restoran, reservasi kedaluwarsa/recovery order terbengkalai, rekonsiliasi reserved vs reservations, available/reserved pada report/card/reorder, WASTE fisik/config untuk item diproses, availability manual menu. Order payment masih diblokir; jangan mengirim langsung ke sales/finalize untuk membayar order karena reservasi belum dikonversi.
