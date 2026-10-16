# Batch 50 — checkout restoran

POST /restaurant/orders/{id}/checkout, If-Match versi order + Idempotency-Key wajib:

```json
{"register_id":"UUID","payments":[{"method":"cash","amount_cents":"5000000"}]}
```

Sesi/CSRF/tenant/modul restaurant/sale.create/cabang wajib. Order OPEN, semua item aktif harus sudah dikirim (NOT_SENT ditolak), CANCELLED dikecualikan. Terminal harus memiliki shift open cabang order. Harga/nama/satuan/modifier/note memakai snapshot order, bukan harga baru. Metadata biaya/prod kategori/tax masih dibaca sesuai shared Finalize.

Checkout mengunci business→shift→order→saldo urut varian. Reservasi tiap line active menjadi consumed dan reserved dikurangi, lalu shared Finalize mengurangi SALE. Efek pembayaran/stock/counter/snapshot/order status/metadata/audit/idempotency dalam transaksi sama. Kegagalan rollback semua. Unique sales.restaurant_order_id memastikan satu sale/order. Retry key/versi/payment sama replay saleResponse; akses cabang tetap dicek sebelum replay.

SC sesuai cabang/order_type dengan include_in_tax_base; config rate/taxbase disnapshot di sales. Unit price final menyertakan modifier_delta, nama/grup/price modifier tersimpan JSON di sale_lines dan tersedia authenticated/public receipt. Order completed/paid/closed_at menyebabkan meja NEEDS_CLEANING; bukan available otomatis.

Batas yang belum: delivery_fee nonzero diblokir karena belum ada allocation/refund/report charge; order diskon belum diterima, customer linkage belum; master varian diarsipkan/nonaktif masih ditolak oleh shared Finalize, track_stock/fractional berubah dapat membutuhkan penanganan khusus. Reservasi hilang/tidak konsisten ditolak, tidak diperbaiki otomatis. Tanpa reservasi line dianggap untracked berdasarkan saat send, tapi penelusuran master perubahan perlu uji khusus.

Tidak menambah fungsi tes baru; build/vet/suite lama -race lulus. Belum uji alur checkout/konkurensi end-to-end; implementasi ini belum siap produksi. Frontend tidak diubah. Semua task B50 [x] berarti implementasi bagian tersebut, bukan AC release lulus.

Rekap checkbox MVP tetap 100 (19 fondasi,47 retail,16 restoran,18 apotek), mencampur UI/QA/infra/task parsial. Tidak mengurangi 10 dari 100 karena batch rincian tidak ekuivalen task induk. Estimasi backend substantif kini sekitar 45–55 kelompok, belum angka audit terverifikasi.
