# B530 — idempotency create order

Migrasi lokal sampai 0065 sudah applied, tidak ada versi baru pada patch ini.

POST /api/v1/restaurant/orders menerima Idempotency-Key opsional maksimal 128 karakter. Server menghash request yang sudah dinormalisasi fee default dan menyimpan response/status bersama order/audit dalam satu transaksi. Payload berbeda/key sama menghasilkan 409. Replay revalidasi cabang aktif dan middleware tenant/modul/izin saat ini, lalu mengembalikan hasil asli meski status order berubah.

Frontend RestaurantOrdersPage dan TablesPage mengirim key dan menyimpan payload sebelum request melalui usePendingMutation, retry memakai snapshot asli, dibatasi retensi 72 jam. Penanda pending legacy yang tidak punya payload/key tetap memerlukan pengecekan manual.

- [x] Implementasi backend/frontend selesai.
- [x] DITUNDA: test/vet/build aplikasi/browser BELUM DIJALANKAN/BELUM LULUS.

Batas: key opsional demi kompatibilitas; hanya klien yang mengirim key mendapat proteksi request replay. Hilang storage/tab bukan jaminan recovery; cek server sebelum membuat ulang. Tidak ada integrasi luar/landing baru.
