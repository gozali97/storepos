# Stok opname — CORE-031

- `POST /api/v1/stock/counts`: `{branch_id}` membuat sesi draft dan snapshot seluruh varian pelacak stok nonarsip di cabang, termasuk saldo nol.
- `GET /api/v1/stock/counts/{id}` membaca status/versi dan snapshot, saldo saat hitung, jumlah fisik, serta timestamp hitung tiap item.
- `PUT /api/v1/stock/counts/{id}/lines/{variant_id}`, `If-Match`: `{counted_qty_milli:"8000"}` merekam 8 unit fisik. Saldo sistem dibaca di bawah row lock pada saat input. Versi sesi bertambah setiap input.
- `POST /api/v1/stock/counts/{id}/post`, `If-Match`: posting hanya bagi izin stock.count_post (Pemilik/Manajer). Penyesuaian atomik dan satu kali, item belum dihitung diabaikan. Tambahkan Idempotency-Key untuk replay setelah timeout: key/ID/versi sama mengembalikan respons posted tersimpan, konflik versi pada key yang sama ditolak IDEMPOTENCY_CONFLICT. Key opsional untuk kompatibilitas klien lama; tanpa key tetap proteksi versi satu-kali. Akses cabang diperiksa sebelum replay. Kunci/histori/audit/saldo commit bersama; kegagalan rollback semua. Tes spesifik replay opname belum ditambah, suite lama dijalankan.

Contoh: snapshot 10, penjualan sebelum hitung 1, fisik 8 → selisih -1 terhadap sistem 9. Jika penjualan berikutnya mengurangi sistem menjadi 8 sebelum posting, hasil posting adalah 7, bukan mengatur saldo kembali ke 8.

Penolakan negatif membatalkan seluruh posting; item perlu dihitung ulang. Perubahan sesi setelah posted ditolak; versi lama ditolak 409. Semua endpoint mengecek tenant/cabang. Gudang boleh input/membaca, tidak boleh posting. Snapshot dan input tidak menghentikan penjualan selain row lock singkat pada item terkait.

Apotek diblokir sampai opname per batch/karantina (APT-013). GET saat ini mengembalikan seluruh item sesi; paginasi diperlukan sebelum sesi katalog besar dipakai. UI dan uji performa 50.000 varian belum dikerjakan.
