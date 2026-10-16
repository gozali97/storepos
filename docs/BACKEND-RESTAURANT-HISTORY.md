# Riwayat pesanan dan master arsip

GET /restaurant/orders?branch_id=UUID&status=open|completed|cancelled|all&order_type=DINE_IN|TAKEAWAY|DELIVERY_MANUAL&from=YYYY-MM-DD&to=YYYY-MM-DD&after=UUID.

Default open, tanpa periode diperbolehkan untuk operasi aktif. Status selain open wajib from/to maksimal 366 hari zona cabang; filter memakai created_at (tanggal pesanan dibuka), bukan tanggal pembayaran/penutupan. Pembayaran restoran belum dibuat sehingga completed tidak dapat dihasilkan melalui checkout saat ini. Cursor id naik, 100 item. Contact_name/phone/queue pada list tetap kosong demi data minimal; detail berizin memuat kontak.

GET /restaurant/areas, /restaurant/tables dan /restaurant/stations dapat include_archived=true. Hanya peran table.manage Allowed (Owner/Manager), bukan kasir/kitchen. Item mengembalikan archived/version. Arsip tidak berarti dihapus atau dapat dipakai pesanan baru; status meja derived bukan izin untuk mengoperasikan meja arsip.

Tidak ada perubahan frontend/migrasi. Tidak membuat fungsi testing baru; suite lama/build/vet lulus. Belum: reaktivasi area/meja, filter closed_at/payment date, daftar arsip modifier, checkout, serta perbaikan data kontak bebas pada note.
