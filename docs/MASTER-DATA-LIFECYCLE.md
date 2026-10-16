# Kebijakan lifecycle master data (CORE-043)

Transaksi final, pembayaran, retur, shift, audit, dan pergerakan stok tidak pernah dihapus. Koreksi memakai void, retur, reversal, atau pergerakan kompensasi. Draft tua dibersihkan worker sesuai retensi.

| Data | Aksi API | Kebijakan |
|---|---|---|
| Produk / varian | PATCH status | `inactive`/`archived`; tidak ada DELETE permanen |
| Satuan kemasan | PUT dengan `archived=true` | Faktor terkunci setelah dipakai; default harus dipindah sebelum arsip |
| Kategori | DELETE | Semantik endpoint adalah arsip + version bump, bukan SQL DELETE |
| Supplier | DELETE | Hard-delete hanya jika tidak direferensikan penerimaan biasa maupun batch apotek; jika dipakai ditolak 409 dan UI harus mengarsipkan |
| Pelanggan | DELETE | Anonimisasi PII; transaksi tetap utuh |
| Cabang | PUT status | Tidak ada DELETE permanen |
| Terminal | PATCH status | Nonaktif ditolak selama shift terbuka; tidak ada DELETE permanen |
| Anggota/pengguna | PUT membership status | Nonaktif, bukan hapus; sesi akun nonaktif ditolak request berikutnya |
| Aturan pajak | DELETE | Arsip, snapshot transaksi lama tetap utuh |
| Area/meja restoran | DELETE | Arsip; area dengan meja aktif atau meja dengan order terbuka ditolak 409 |
| Metode bayar | enum/snapshot pembayaran | Tidak ada master DELETE pada MVP |
| Barcode | DELETE | Identitas yang boleh dilepas; transaksi menyimpan snapshot SKU/nama/satuan |
| Undangan | DELETE | Revoke token sementara |
| Link struk | DELETE | Revoke bearer link, transaksi tidak dihapus |
| Kontak order | DELETE | Erasure PII, order tetap utuh |
| Baris order belum dikirim | DELETE | Draft operasional; setelah dikirim memakai cancel + audit |
| Data contoh onboarding | DELETE | Arsip produk contoh, bukan hard-delete |

`master_lifecycle_test.go` menginventaris semua rute DELETE. Rute DELETE baru harus ditambah ke allowlist dengan keputusan lifecycle eksplisit; data historis utama dilarang memiliki endpoint DELETE.

Batasan: penghentian seluruh tenant mengikuti retensi eksternal/legal dan sengaja tidak diimplementasikan di sini. Hak SQL deployment (`REVOKE`) tetap lapisan terpisah.
