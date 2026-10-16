# Void transaksi — CORE-027

`POST /api/v1/sales/{sale_id}/void`

Wajib: cookie sesi, CSRF, X-Business-ID, izin sale.void, akses cabang, dan Idempotency-Key.

```json
{
  "reason": "Salah input",
  "approval": {"email": "atasan@example.test", "pin": "123456"}
}
```

- Persetujuan harus dari Pemilik/Manajer aktif yang berbeda dari pelaku.
- Hanya transaksi completed dengan shift asal masih terbuka.
- Membalik stok dari pergerakan SALE historis, bukan pengaturan produk saat ini.
- Pembayaran menjadi reversed; pembayaran nontunai ditandai manual_refund_required. Sistem tidak mengirim dana ke provider.
- Penjualan voided tidak lagi dihitung dalam penerimaan tunai/penjualan ringkasan shift. Nomor struk tetap tersimpan dan tidak digunakan ulang.
- Kunci yang sama mengembalikan hasil tersimpan tanpa membalik stok dua kali. PIN masih diverifikasi pada replay.
- Shift tertutup: HTTP 409 INVALID_STATE_TRANSITION, gunakan retur (backend retur belum tersedia).

Migrasi down 0007 sengaja gagal bila histori void sudah ada; tidak menghapus histori koreksi secara diam-diam.
