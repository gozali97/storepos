# Penetapan HPP awal — §13.11

PUT /api/v1/stock/variants/{id}/opening-cost, Idempotency-Key wajib, sesi/CSRF/business.manage/Pemilik dan cabang:

```json
{"branch_id":"UUID","average_cost_cents":"200000","reason":"Biaya saldo awal dari faktur"}
```

HPP per satuan dasar dalam string sen. Saldo harus positif, produk pelacak stok nonarsip/cabang aktif, HPP sekarang NULL. HPP diketahui ditolak 409, tidak boleh ditimpa. Saldo dikunci agar penerimaan/penjualan tidak berlomba; penetapan dan audit serta kunci replay berada dalam satu transaksi. Qty stok tidak berubah, jadi tidak membuat stock_movements tambahan. Alasan wajib maksimal 200 karakter. HPP nol hanya jika secara eksplisit dikirim sebagai biaya nol.

Setelah HPP awal terisi, penerimaan berbiaya yang sebelumnya diblokir ErrUnknownCost dapat diposting untuk memperbarui moving average. Penjualan berikutnya memiliki snapshot HPP; snapshot lama yang tidak diketahui tetap NULL dan tidak diisi dengan HPP sekarang.

Key sama/payload sama mengembalikan hasil tersimpan; key sama/berbeda ditolak. Pemilik tetap perlu mengonfirmasi sumber biaya; sistem tidak dapat memvalidasi faktur eksternal. Tidak ada perubahan frontend/migrasi pada batch ini. Tidak menambah fungsi tes baru; build/vet dan suite tes lama dijalankan. Verifikasi HTTP spesifik endpoint dan konkurensi HPP awal belum dilakukan.
