# B520 — 20 fungsi daftar & header pesanan restoran

Migrasi lokal: make migrate-up dijalankan dan status sampai 0065 sudah diterapkan. Tidak ada migrasi baru pada fitur ini.

**Testing/vet/build aplikasi/browser tetap ditunda.**

- [x] B520-01 Route lazy-loaded /pesanan dan menu daftar pesanan restoran, tautan dari detail pesanan. **Implementasi; BELUM DIUJI.**
- [x] B520-02 Pembatasan daftar untuk restoran dengan sale.create (Pemilik/Manajer/Kasir), reset state per bisnis/pengguna/cabang. **Implementasi; BELUM DIUJI.**
- [x] B520-03 Daftar semua pesanan terbuka cabang lintas terminal, bukan hanya pesanan dari grid meja. **Implementasi; BELUM DIUJI.**
- [x] B520-04 Filter status open/completed/cancelled/all dari server. **Implementasi; BELUM DIUJI.**
- [x] B520-05 Filter jenis DINE_IN/TAKEAWAY/DELIVERY_MANUAL. **Implementasi; BELUM DIUJI.**
- [x] B520-06 Filter periode lokal cabang untuk histori non-open, reset cursor ketika filter berubah. **Implementasi; BELUM DIUJI.**
- [x] B520-07 Pagination cursor order dengan halaman pertama/berikutnya dan refresh manual. **Implementasi; BELUM DIUJI.**
- [x] B520-08 Tampilan ringkas antrean/kanal, ongkir BigInt sen, waktu zona cabang dan status pembayaran tanpa menampilkan nama/telepon pada daftar. **Implementasi; BELUM DIUJI.**
- [x] B520-09 Buka detail order dari daftar untuk melanjutkan order open atau melihat order final. **Implementasi; BELUM DIUJI.**
- [x] B520-10 Form pembuatan TAKEAWAY tanpa meja, nama/antrean opsional. **Implementasi; BELUM DIUJI.**
- [x] B520-11 Form pembuatan DELIVERY_MANUAL dengan kanal bebas, kontak opsional, ongkir dan catatan. **Implementasi; BELUM DIUJI.**
- [x] B520-12 Validasi header bersama: tipe/length/telepon/ongkir, nominal rupiah ke sen tepat tanpa float. **Implementasi; BELUM DIUJI.**
- [x] B520-13 Proteksi klik ganda pembuatan order dengan ref selama request aktif. **Implementasi; BELUM DIUJI.**
- [x] B520-14 Penanda pembuatan pending di sessionStorage per cakupan, memblokir pengiriman ulang setelah timeout/refresh sampai server diperiksa. **Implementasi; BELUM DIUJI.**
- [x] B520-15 Pemeriksaan daftar order terbuka dari banner hasil tak pasti; reset manual memerlukan konfirmasi operator. **Implementasi; BELUM DIUJI.**
- [x] B520-16 Detail order memuat header/status/versi server, menampilkan tipe, status payment, antrean, kanal, ongkir dan catatan. **Implementasi; BELUM DIUJI.**
- [x] B520-17 Form edit note/header dengan nama/antrean takeaway dan kanal/ongkir/telepon delivery sesuai jenis, memakai If-Match. **Implementasi; BELUM DIUJI.**
- [x] B520-18 Erasure kontak/antrean khusus Pemilik dengan konfirmasi checkbox, tanpa menghapus dokumen order. **Implementasi; BELUM DIUJI.**
- [x] B520-19 Pembatalan seluruh order melalui form alasan dan If-Match, menjelaskan item dapur harus dibatalkan berotorisasi dahulu. **Implementasi; BELUM DIUJI.**
- [x] B520-20 Blok workspace mutasi item/payment pada order final, reload header setelah perubahan atau konflik untuk menjaga versi/status terbaru. **Implementasi; BELUM DIUJI.**

Berkas: RestaurantOrdersPage.jsx, RestaurantOrderHeader.jsx, restaurantHeaders.js, OrderPage.jsx, App.jsx, AdminLayout.jsx. Endpoint lama dipakai, tidak mengubah landing/provider luar/backend runtime.

Batas: create order tanpa meja belum idempoten server; penanda pending melindungi retry pada tab yang sama, bukan lintas tab atau storage terhapus. Operator harus memeriksa server sebelum mengulangi create. Penghapusan kontak butuh konfirmasi, namun salinan backup mengikuti retensi eksternal. Tidak ada klaim uji perilaku UI/konflik/timeout sudah lulus.

- [x] **DITUNDA: testing B520 BELUM DIJALANKAN/BELUM LULUS.**
