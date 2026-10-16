# Retur dan refund — CORE-028

`POST /api/v1/sales/{sale_id}/returns`, dengan cookie sesi, CSRF, X-Business-ID, Idempotency-Key dan izin sale.return.

```json
{
 "register_id": "UUID terminal refund tunai",
 "reason": "Barang dikembalikan",
 "lines": [{"sale_line_id": "UUID baris transaksi", "qty_milli": "1000", "condition": "restock"}],
 "approval": {"email": "atasan@example.test", "pin": "123456"}
}
```

Persetujuan wajib bagi peran dengan izin bersyarat (kasir/apoteker). Nilai diambil dari snapshot line_total_cents, sehingga pajak inklusif tidak ditambahkan dua kali. Setiap refund dibulatkan ke rupiah; retur sisa terakhir menerima sisa nilai yang belum direfund. Pembulatan tunai dikembalikan hanya saat retur penuh.

- `restock` menambah stok jual; `damaged` mencatat RETURN_DAMAGED tanpa menambah saldo jual.
- Kuantitas dibatasi terjual dikurangi retur sebelumnya. Mutasi paralel dikunci pada transaksi asal.
- Refund tunai memerlukan shift terbuka pada terminal cabang asal dan mengurangi kas diharapkan shift refund.
- Refund metode nontunai asli dicatat manual; tidak mengirim dana ke provider.
- Void sesudah retur parsial/penuh diblokir untuk mencegah pengembalian dana/stok ganda.
- Status retur tersimpan terpisah sebagai none/partial/full; status penjualan tetap completed.

## Batas saat ini

Transaksi split tender diblokir untuk retur sampai alokasi refund per metode tersedia. Apotek diblokir sampai alur batch/karantina tersedia. Batas hari retur configurable tersedia melalui operational-policy, diuji memakai tanggal lokal WITA; 0 tanpa batas. Rincian neto/pajak/SC dan HPP snapshot kini tersedia untuk retur baru. Pencarian transaksi/baris melalui nomor struk dan penyelesaian split tender/apotek tetap terbuka. UI dikerjakan agent terpisah. Karena itu CORE-028 belum lengkap.
