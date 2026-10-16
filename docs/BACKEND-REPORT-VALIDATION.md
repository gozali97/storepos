# Validasi rekonsiliasi dan performa laporan

## CORE-036

TestDailyReportReconcilesAllClosedShifts membandingkan penjualan neto/jumlah transaksi harian dengan tiga ringkasan shift tertutup pada cabang yang sama (kas awal nol, semua pembayaran cash, tanpa diskon/pajak/retur). Perbandingan menggunakan completed_at dalam zona Asia/Jakarta, batas mulai inklusif dan akhir eksklusif. Transaksi tepat awal hari termasuk; transaksi tepat awal hari berikutnya dikecualikan.

TestPaymentReportReconcilesShiftAndRefundDate menguji penerimaan/refund tunai neto dan tanggal lokal refund berbeda. TestSalesReportTaxReturnsAndHistoricalMissing menguji pajak inklusif/eksklusif, tanggal retur dan komponen histori tidak lengkap.

Belum ada satu pengujian gabungan rekonsiliasi seluruh komponen pajak/diskon/refund/void dengan shift lintas tengah malam. Jangan membandingkan total seluruh shift dengan laporan satu hari bila shift mengandung transaksi dua hari: ringkasan harus dipecah menurut tanggal kejadian atau periode diperluas.

## NFR-004

TestSalesReport5000TransactionsPerformance memakai DB sementara berisi 5.000 transaksi lengkap dengan snapshot baris, lima eksekusi query penjualan, dan assert <=3 detik. Pemeriksaan count memastikan query tidak mengabaikan fixture.

Perintah:

```
cd backend
go test -race -count=1 -v -run 'TestDailyReportReconciles|TestSalesReport5000' ./internal/sales
```

Pengukuran lokal saat implementasi (PostgreSQL 16, -race): 843.87 ms, 844.80 ms, 13.54 ms, 12.79 ms, 12.49 ms. Ini pengukuran query lokal, bukan SLA API produksi atau p95. Test dilewati dengan -short dan DB integration dapat dilewati jika DB tidak tersedia; periksa output PASS/SKIP.

Belum: benchmark pembayaran/koreksi/stok/performa, jaringan/serialisasi API, multitenant besar, beban bersamaan, variasi baris banyak per transaksi, ekspor >100.000 hasil dari DB, serta profil memori artifact. Fase 2.11 tetap memiliki butir terbuka.
