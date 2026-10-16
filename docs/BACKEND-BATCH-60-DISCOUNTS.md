# Batch 60 — diskon pesanan restoran

PUT /restaurant/orders/{id}/discount, If-Match versi ORDER, {discount:{kind:"none|amount|percent",value:"integer"},reason:"..."}. Amount dalam sen, percent basis points. PUT /restaurant/orders/{id}/lines/{line}/discount payload sama. None/value 0 menghapus diskon. Hanya OPEN/item bukan cancelled; versi naik, audit. Alasan disimpan pada HEADER order (perubahan diskon berikutnya mengganti alasan bersama). Penyimpanan draft diskon bukan otorisasi pembayaran.

GET /restaurant/orders/{id}/discounts membaca header dan seluruh line diskon aktif satu versi. GET bill menghitung diskon line/transaction eligible sebelum SC/tax dan menolak nominal >basis, bukan cap diam-diam. Bill tidak memverifikasi approval/batas peran, label BELUM LUNAS tetap berlaku.

POST checkout menerima approval:{email,pin} opsional. PIN diverifikasi di luar transaksi bisnis agar kegagalan persisten; limit policy/peran diperiksa oleh Finalize saat bayar, approval di atas limit + reason wajib. PIN tidak disimpan pada order/sale/audit; hash body idempotency mencakup payload/versi. Replay masih memverifikasi PIN seperti endpoint keuangan lain. Alasan/pemberi/penyetuju sale mengacu pelaku checkout dan approval saat checkout, bukan actor draft discount. Alokasi diskon snapshot tersedia pada sale/report.

Belum: alasan/actor diskon terpisah tiap baris, pratinjau kebutuhan approval tepat, diskon pada delivery fee, dan uji khusus checkout concurrency/mixed-tax/SC/discount. Ongkir nonzero tetap diblokir pada checkout, tidak diabaikan.

Migrasi 0054. Tidak membuat fungsi testing baru, build/vet/suite lama backend -race lulus, frontend tidak diubah. Sesudah merapikan 5 task induk lama: 95 checkbox MVP terbuka (19 fondasi/47 retail/11 restoran/18 apotek); total seluruh dokumen 129 terbuka termasuk Fase0/rilis/backlog. Ini hitungan checklist, bukan hitungan backend eksak. Perkiraan backend substantif 40–50 kelompok, harus dinormalisasi sebelum dianggap jumlah pasti.
