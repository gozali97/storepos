# Restore master dan histori modifier/tiket

POST /restaurant/areas/{id}/restore dan /tables/{id}/restore, If-Match, Owner/Manager + cabang/modul. Master harus archived, cabang aktif. Table memerlukan area aktif terlebih dahulu; kode/name uniqueness tetap mencegah nama berbenturan. Restore tidak menghapus histori/cleaning state; meja mungkin tetap NEEDS_CLEANING. Audit menyimpan tindakan dan versi.

GET /restaurant/products/{id}/modifiers?include_archived=true menampilkan grup arsip dan opsi nonaktif. Hanya pengelola table.manage Allowed. Default hanya grup/opsi aktif; payload sekarang memuat archived/active. Reaktivasi grup lewat PUT modifier-group archived=false; opsi lewat PUT option active=true. Snapshot pesanan lama tidak berubah.

Migrasi 0051 menambah station_name_snapshot di kitchen_tickets. Kirim baru menyimpan nama saat routing; detail tiket memakai snapshot dan station_snapshot_missing=true untuk histori lama NULL, bukan mengklaim nama terkini sebagai histori. Frontend perlu fallback berlabel saat snapshot missing.

Tidak ada fungsi tes baru; build/vet/suite lama lulus. Frontend tidak diubah. Checkout/restock waste/recovery dan apotek/platform tetap terbuka.
