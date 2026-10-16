# B570 — 5 task induk implementasi MVP

Status: implementasi selesai pada scope MVP; testing ditunda, bukan bukti rilis.

- [x] OpenAPI /api/v1
- [x] Retur/refund
- [x] Query laporan
- [x] Laporan stok
- [x] Ekspor CSV

## Perubahan nyata batch ini
- Menambahkan schema enam operasi yang terakhir skeleton; generator mendukung media request/response selain JSON (CSV/SSE).
- Riwayat dokumen retur berhalaman dan detail void ditampilkan dari endpoint lama, lengkap alasan/pelaku/penyetuju/waktu dan nullable rounding historis.
- Pilihan kondisi retur apotek secara eksplisit diberi label masuk karantina batch asal.

## Penyelarasan scope task induk
CORE-039 hanya meminta saldo varian/batch, nilai estimasi, minimum dan movement periode; saldo historis bukan syaratnya. CORE-044 meminta kesamaan angka ekspor, audit, dan pemrosesan latar >100k; tidak mewajibkan object storage. Query jenis laporan berbeda boleh berbeda, selama JSON/CSV jenis yang sama mengambil data dari sumber query sama. Tidak menandai tambahan ini sebagai fitur yang dibuat.

## Batas belum diuji
Tidak menjalankan test, vet, build aplikasi/browser atau migration. Semua operasi memiliki implementasi kontrak, bukan semua kontrak diverifikasi. Validator lama masih subset, schema baru raw CSV/SSE/nullable/oneOf belum dijalankan lewat validator penuh. UI histori memakai endpoint terotorisasi lama dan belum diuji interaksi. PII/approval belum direfaktor oleh batch ini. Ekspor batch/kitchen masih sinkron bounded, export artifact utama <=50MiB PostgreSQL. Kesiapan rilis dan AC tetap bukan selesai.

- [x] DITUNDA — Testing B570 BELUM DIJALANKAN/BELUM LULUS.
