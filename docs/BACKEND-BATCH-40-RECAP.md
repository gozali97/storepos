# Batch 40 — 10 task dan rekap

1–2 GET /products/{id}/tax-rule dan /categories/{id}/tax-rule: Owner, tax_rule_id/versi master.
3 POST /tax-rules/{id}/restore, If-Match: reaktivasi aturan archived, audit.
4–7 GET /sales/{id}: snapshot nama/tarif/mode pajak per baris; gross/diskon/alokasi/neto/tax/SC; nominal sudah direfund; verification pembayaran. Tidak mengirim HPP tanpa izin.
8 POST /public/receipt: rincian tax-rule/mode/rate/diskon/tax/SC per baris, tetap tanpa HPP/HP/catatan.
9 GET /sales/{id}/returns?after=UUID: riwayat 100/next_cursor, jumlah/pembulatan/alasan/pelaku/approver/waktu.
10 GET /sales/{id}/void: status/pelaku/approver/alasan/waktu, null jika belum void.

Akses histori koreksi sale.return dan cabang, no-store. Snapshot tidak dibaca dari master terkini. Tidak ada migrasi baru, fungsi tes baru atau perubahan frontend. Build/vet/suite lama -race lulus. Endpoint baru belum diuji khusus.

## Rekap checkbox sesudah merapikan 7 checkbox implementasi yang sudah ada

Fase 1 terbuka 19; Fase 2 47; Fase 3 16; Fase 4 18: total 100 checkbox MVP. Angka ini bukan 100 task backend: mencampur UI/QA/infra/legal, sebagian task duplikat dan implementasi parsial. Batch B10/B20/B30/B40 merupakan rincian fitur, tidak dikurangkan satu-per-satu dari task induk yang masih memiliki sisa.

Total seluruh dokumen: 134 checkbox terbuka (11 Fase 0, 100 MVP, 13 kesiapan rilis, 10 backlog). Estimasi kelompok backend yang masih substantif sekitar 50–60, bukan hitungan terverifikasi: checkout restoran adalah satu kelompok besar yang dapat memerlukan satu batch tersendiri; apotek hampir seluruhnya belum, platform MFA/support belum, registrasi/verifikasi/integrasi belum. Jangan menganggap 40 task rincian berarti 40 kebutuhan PRD lengkap telah selesai.
