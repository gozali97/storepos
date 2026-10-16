# Pengeluaran operasional — CORE-041

POST /expenses dengan Idempotency-Key:

```json
{"branch_id":"UUID","expense_date":"2026-10-13","category":"Listrik","amount_cents":"30000000","method":"outside_cash","note":"Tagihan bulan ini"}
```

Metode shift_cash memerlukan register_id terminal dengan shift terbuka pada cabang yang sama. Expense dan cash_movements out ditulis bersama dalam satu transaksi. Outside_cash tidak mengubah kas shift maupun HPP. Tidak ada lampiran/pembukuan akuntansi.

Izin expense.manage dan akses cabang wajib. Kasir hanya boleh shift_cash, wajib approval `{email,pin}` karena izinnya bersyarat. Semua kas keluar di atas Rp500.000 juga wajib approval sesuai CORE-025. Kunci sama/isi sama replay tanpa menggandakan expense atau kas keluar; PIN persetujuan masih diverifikasi pada replay.

GET /expenses?branch_id=UUID&from=YYYY-MM-DD&to=YYYY-MM-DD&after=UUID mengembalikan 100 item dan next_cursor. Periode mengacu pada tanggal dokumen yang dimasukkan, bukan waktu posting. Kategori berupa teks pengguna, belum master kategori. Nominal string integer sen.

Belum: edit/void pengeluaran, lampiran, ringkasan agregat/ekspor laporan dan UI. Jangan mengoreksi expense dengan UPDATE langsung karena harus membalik kas terkait; belum ada API koreksi.
