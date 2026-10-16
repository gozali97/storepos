# Task List — StorePOS

Disusun dari PRD.md v0.1 dan mengikuti tahapan roadmap §22. Setiap task merujuk ID kebutuhan di PRD (CORE-xxx, RTL-xxx, RST-xxx, APT-xxx, LP-xxx, NFR-xxx, §x).

Stack: Go (modular monolith), React, PostgreSQL.

Legenda: `[ ]` belum · `[x]` implementasi selesai (lihat batas batch) · `[x] DITUNDA` testing tidak dikerjakan/belum lulus · `[x] DILEWATI / TERBLOKIR` ditutup administratif, bukan implementasi selesai · **(blok)** = menunggu keputusan TBK/VAL

## Status ringkas

### Penutupan administratif blocker & perbaikan lokal (B600)
- [x] Sisa 15 task induk MVP yang terblokir ditandai DILEWATI / TERBLOKIR atas permintaan pengguna, bukan diimplementasikan/lulus. Ledger docs/BLOCKED-MVP-LEDGER.md menjaga alasan dan risiko tiap item.
- [x] Perubahan kata sandi: setelah verifikasi di luar transaksi, lock user dan cek hash/status masih sama sebelum update; penolakan 409/401 tidak diubah menjadi 500. Menghindari perubahan paralel menimpa password baru.
- [x] Batch valuation mengubah operand ke numeric sebelum perkalian qty*cost agar tidak overflow bigint sebelum agregasi.
- [x] Pengaturan organisasi di-remount per bisnis/pengguna/cabang agar cursor terminal/nonaktif state lama tidak terbawa; nama metode manual divalidasi UTF-8/nonblank.
- [x] DITUNDA — testing/vet/build aplikasi/browser B600 BELUM DIJALANKAN/BELUM LULUS. Hanya pembacaan kode/gofmt dan perubahan dokumentasi; tidak ada migrasi baru.

Nol checkbox induk MVP terbuka setelah penutupan administratif TIDAK berarti semua fitur selesai atau siap produksi. Blocker masih belum dibuat/divalidasi, testing tetap ditunda. Tidak mengubah landing/provider luar; tidak membuat RLS semu yang bisa membypass tenant karena role owner.

### Akses resep terbatas & penerapan metode bayar (B590)
- [x] Migrasi 0066 payment_method_settings diterapkan lokal; runtime penjualan dapat membaca konfigurasi metode manual. migrate-up/status dijalankan atas keputusan melanjutkan perubahan lokal yang memerlukan kolom ini.
- [x] GET /pharmacy/sales/{id}/prescription: Apoteker saja, izin prescription_verify, tenant/cabang aktif dari konteks, transaksi baca+audit fail-closed, tidak memasukkan nilai resep ke audit.
- [x] Respons no-store/private/no-cache/no-referrer; detail sales/struk umum tidak ditambahi referensi resep. Kontrak endpoint implemented-not-tested.
- [x] UI referensi di detail penjualan hanya apoteker apotek, explicit buka/close, tidak disimpan local/sessionStorage, auto hide saat tab hidden/beforeprint, request dibatalkan saat unmount.
- [x] DITUNDA — test/vet/build aplikasi/browser B590 BELUM DIJALANKAN/BELUM LULUS. Migrasi 0066 yang diperlukan runtime diterapkan; gofmt dan generator dokumen saja selain itu.

Task induk resep TETAP terbuka karena field minimum/regulasi eksternal belum valid; tidak menutup lima parent yang terblokir hanya demi target. Task parent tersisa juga memuat backup, OTP/email, RLS/role deployment, MFA/support, keputusan pindah meja, validasi dan LP eksternal. Landing/provider luar tidak diubah.

### Penuntasan pengaturan organisasi lokal (B580)
- [x] GET/PUT /settings/payment-methods, default empat metode manual, version/If-Match, validasi minimal satu aktif/nama/kode unik, audit dan permission Pemilik untuk perubahan.
- [x] Finalize bersama (retail/restoran/apotek) menolak metode nonaktif pada penjualan baru; refund asal tidak dipaksa memakai konfigurasi baru. Nama tampilan tidak mengganti kode historis.
- [x] UI PaymentMethodSettings di Settings, selector Cashier/Order memakai metode aktif, pilihan default yang nonaktif wajib diganti sebelum bayar; backend tetap otoritatif.
- [x] Daftar seluruh terminal termasuk nonaktif, pagination, refresh, nonaktif/aktif ulang dari halaman Pengaturan (tidak bergantung state sesi lokal saja).
- [x] Kontrak dua endpoint documented implemented-not-tested; migrasi 0066 payment_methods_version diterapkan lokal pada B590.
- [x] DITUNDA — testing/vet/build aplikasi/browser B580 BELUM DIJALANKAN/BELUM LULUS. Gofmt dan generator dokumen saja; tidak menjalankan migrasi/deployment.

Permintaan 5 task induk belum dapat dipenuhi sah karena hampir semua sisa parent terblokir provider/infra/legal/landing di luar scope. B580 menutup 1 parent yang bisa dikerjakan lokal; tidak menandai 4 parent terblokir sebagai selesai. Metode hanya rename/enable empat kode MVP, bukan custom-code baru; migrasi 0066 wajib diterapkan sebelum runtime baru.

### Penuntasan 5 task induk MVP (B570) — implementasi, bukan kelulusan tes
- [x] OpenAPI: enam schema operasi tersisa (impor commit/preview/template, SSE restoran, laporan cancellation, leads) + generator media request/response CSV/SSE. Semua 209 operasi memiliki implementasi kontrak, 99 pernah verified +110 belum diuji. Bukan klaim semua schema sempurna/validated.
- [x] Retur/refund: komponen histori dokumen retur berhalaman dan detail void pada halaman transaksi; label retur apotek eksplisit karantina. QR/struk digital/split refund memakai implementasi lama.
- [x] Query laporan: diselaraskan dengan PRD—satu sumber query per jenis laporan untuk JSON/CSV, bukan memaksa laporan berbeda menjadi satu SQL.
- [x] Stok CORE-039: saldo/nilai saat ini, minimum, periode movement dan batch selesai pada cakupan implementasi. Saldo/nilai historis tidak diwajibkan CORE-039, tidak dinyatakan dibuat.
- [x] CSV CORE-044: query sama, audit, worker ekspor utama, status/history/download; storage PostgreSQL bounded dipakai, object storage eksternal bukan syarat dan tetap dilewati.
- [x] DITUNDA — testing/vet/build aplikasi/browser B570 BELUM DIJALANKAN/BELUM LULUS. Hanya gofmt dan generator dokumentasi; tidak ada migrasi baru atau deployment.

Lima task induk ditutup sebagai IMPLEMENTASI MVP, bukan AC lulus/siap rilis. Rincian docs/BATCH-570-FIVE-PARENT-TASKS.md. Landasan scope PRD CORE-028/039/044 dan §13.10/16.4; tidak menghapus risiko historis, batas artifact atau kebutuhan pengujian. Landing/runtime leads dan provider luar tidak diubah.

### Batch 20 tugas laporan operasional (B560) — testing ditunda
- [x] B560-01 Route /restoran/laporan lazy-loaded dengan menu dan tautan dari daftar pesanan; role Pemilik/Manajer dan modul restoran. **Implementasi; BELUM DIUJI.**
- [x] B560-02 Reset state laporan restoran per bisnis/pengguna/cabang serta cabang aktif wajib. **Implementasi; BELUM DIUJI.**
- [x] B560-03 Tab pembatalan item dan pesanan terlalu lama dari endpoint lama. **Implementasi; BELUM DIUJI.**
- [x] B560-04 Filter pembatalan periode maksimal 366 hari dan filter pelaku UUID opsional. **Implementasi; BELUM DIUJI.**
- [x] B560-05 Waktu pembatalan ditampilkan memakai zona cabang, bukan zona browser secara tak sengaja. **Implementasi; BELUM DIUJI.**
- [x] B560-06 Detail menu/varian/jumlah satuan dasar memakai snapshot dan BigInt milli-unit. **Implementasi; BELUM DIUJI.**
- [x] B560-07 Modifier snapshot dirender sebagai teks ter-escape dan status asal pembatalan ditampilkan. **Implementasi; BELUM DIUJI.**
- [x] B560-08 Penjelasan potensi waste sebagai indikator, bukan bukti pemusnahan fisik. **Implementasi; BELUM DIUJI.**
- [x] B560-09 Alasan, ID pelaku, dan penyetuju pembatalan ditampilkan tanpa mengambil PII pelanggan. **Implementasi; BELUM DIUJI.**
- [x] B560-10 Nilai menu BigInt sen dilabeli harga jual snapshot, bukan HPP/kerugian. **Implementasi; BELUM DIUJI.**
- [x] B560-11 Unduh CSV pembatalan memakai filter sama; lock request, error handling dan revoke object URL. **Implementasi; BELUM DIUJI.**
- [x] B560-12 Link dari pembatalan ke dokumen pesanan asal untuk penelusuran. **Implementasi; BELUM DIUJI.**
- [x] B560-13 Laporan order lama memakai ambang jam integer 1–720 yang divalidasi dan cursor reset saat berubah. **Implementasi; BELUM DIUJI.**
- [x] B560-14 Daftar order lama menampilkan jenis/meja/created_at lokal/umur dari server/versi dan link periksa order. **Implementasi; BELUM DIUJI.**
- [x] B560-15 Pagination order lama pertama/berikutnya dan refresh, tidak otomatis cancel/close order. **Implementasi; BELUM DIUJI.**
- [x] B560-16 Route /rekonsiliasi lazy-loaded dengan menu/link audit, hanya Pemilik/Manajer berizin audit. **Implementasi; BELUM DIUJI.**
- [x] B560-17 API rekonsiliasi menambah started/completed run, count khusus tenant dan date_basis UTC, tanpa count tenant lain. **Implementasi; BELUM DIUJI.**
- [x] B560-18 UI membedakan worker belum mulai, belum selesai, run tanpa temuan, dan hasil terpotong; hasil kosong bukan otomatis bersih. **Implementasi; BELUM DIUJI.**
- [x] B560-19 Filter jenis stock/payment/reservation, tabel entitas/expected/actual/delta BigInt integer, nilai desimal ditampilkan asli. **Implementasi; BELUM DIUJI.**
- [x] B560-20 OpenAPI metadata rekonsiliasi disinkronkan; halaman readonly menjelaskan unit sen/milli dan koreksi wajib lewat dokumen sah, loading/error/empty/refresh tersedia. **Implementasi; BELUM DIUJI.**
- [x] DITUNDA atas permintaan pengguna — testing B560 BELUM DIJALANKAN/BELUM LULUS, tanpa test/vet/build aplikasi/browser/migrasi. Gofmt dan generator dokumentasi saja.

Laporan pembatalan/aging restoran dan rekonsiliasi readonly dihubungkan ke UI. Metadata run ditambahkan agar hasil kosong tidak otomatis disebut bersih. Rincian docs/BATCH-560-OPERATIONAL-REPORTS.md. Tidak mengubah landing/provider luar/schema database. Task induk terkait validasi/performa tetap terbuka.

### UI impor katalog CSV (B550)
- [x] migrate-up/status dijalankan: seluruh versi sampai 0065 applied; runner menyatakan tidak ada migrasi baru.
- [x] /impor-katalog lazy-loaded, route aplikasi/preview, tautan Products/onboarding; khusus retail Pemilik/Manajer, cabang wajib dan reset scope per user/bisnis/cabang.
- [x] Unduh template server, pilih file maksimal 5MiB, decode UTF-8 fatal, tolak kosong, stale file read tidak menimpa file baru; penjelasan harga integer sen/qty milli/HPP.
- [x] API client mendukung rawBody string text/csv tanpa body JSON; JSON caller lama tetap memakai jalur yang sama. Pending mutation meneruskan rawBody asli.
- [x] Preview server menampilkan jumlah/valid/error row-field-message tanpa perubahan data; ganti file menghapus preview, abort request saat unmount.
- [x] Commit wajib preview valid dan konfirmasi pengguna; snapshot CSV+key disimpan sebelum request, refresh/retry memakai payload/key yang sama, aksi baru diblokir saat pending (termasuk storage error/expired sesuai B290).
- [x] DITUNDA — testing/vet/build aplikasi/browser B550 BELUM DIJALANKAN/BELUM LULUS. Migrasi/status saja dijalankan sesuai permintaan; tidak ada migrasi baru.

Batas: sessionStorage bisa tidak cukup untuk file CSV besar (gagal simpan berarti request tidak dikirim); data recovery tab bisa hilang saat tab ditutup/storage dibersihkan, periksa katalog server sebelum impor ulang. Maks 5.000 baris, satu produk/varian per row; kategori/multi-varian/apotek/restoran belum didukung backend import. RawBody API change dan JSX belum diuji. Landing/provider luar tidak diubah.

### Keamanan akun & riwayat ekspor (B540)
- [x] migrate-up/status dijalankan atas permintaan pengguna: sampai 0065 sudah applied, tidak ada versi baru pada B540.
- [x] `/keamanan`: daftar sesi akun sendiri berhalaman, penanda sesi saat ini, browser/IP/login/last seen/expiry, reload manual dan 60 detik dengan cleanup timer.
- [x] Konfirmasi inline pencabutan sesi lain, lock request ganda, penjelasan sesi saat ini tetap aktif, refresh sesudah sukses/gagal, pesan hasil belum pasti; link ganti kata sandi.
- [x] `/ekspor`: riwayat milik pengguna/bisnis/cabang berizin, cursor, status job, jumlah baris unknown vs nol, expiry, detail error_code, refresh server, unduh CSV hanya completed/nonexpired dengan revalidasi API.
- [x] Dua halaman lazy-loaded, menu dan route aplikasi/preview ditambahkan; link silang dari Settings dan Reports. Tidak mengubah backend/data/landing/integrasi luar.
- [x] DITUNDA — testing/vet/build aplikasi/browser B540 BELUM DIJALANKAN/BELUM LULUS. Hanya migrasi/status dijalankan (sesuai permintaan), implementasi UI ditulis tanpa klaim validasi.

Batas: waktu sesi/job ditampilkan dalam zona perangkat; user-agent/IP hanya petunjuk, bukan fingerprint terpercaya. Riwayat export membutuhkan worker berjalan; refresh masih manual, bukan polling. Download tetap dibatasi retensi dan izin server.

### Idempotency pembuatan order (B530)
- [x] Migrate-up dan status dijalankan: seluruh versi sampai 0065 applied, tidak ada migrasi baru tertunda.
- [x] POST /restaurant/orders menerima key opsional (legacy kompatibel), hash payload server, penyimpanan hasil+order+audit atomik; replay mengembalikan order_id/status HTTP asli (201 baru/200 resume).
- [x] Validasi tenant/modul/izin/cabang aktif tetap sebelum replay; key beda payload 409, key maksimal 128. Riwayat key mengikuti retensi 72 jam backend.
- [x] Create takeaway/delivery menggunakan usePendingMutation: payload+key disimpan sebelum request, retry setelah refresh dengan key asli, aksi baru diblokir saat pending, tanpa menyimpan PIN.
- [x] Buka meja dari grid juga mengirim key dan retry asli; cleaning/open lain diblokir saat pending. Penanda pending legacy tetap dipertahankan untuk order yang dibuat sebelum perbaikan.
- [x] DITUNDA — testing/vet/build aplikasi/browser B530 BELUM DIJALANKAN/BELUM LULUS. Hanya migrasi/status yang diminta pengguna, gofmt, dan pembangkitan dokumentasi dijalankan.

Tidak ada migrasi/schema baru. Kontrak create order tetap implemented-not-tested dan mendokumentasikan header key opsional. Risiko: klien lama yang tidak mengirim key tetap tidak mendapat proteksi duplikasi takeaway/delivery; hapus storage/tab tetap perlu cek status server. Landing/integrasi luar tidak diubah.

### Redesign panduan awal
- [x] `/mulai`: stepper vertikal tanpa nomor ganda, penjelasan tiap langkah, form responsif, aksi simpan/lanjut/lewati yang jelas, status memuat/gagal/tersimpan/selesai dan pemuatan langkah pertama otomatis. Tes UI/onboarding dan build lulus; interaksi API serta verifikasi browser belum.

### Redesign dashboard
- [x] `/dashboard` mengikuti Soft UI biru: sambutan dan konteks cabang/terminal, ringkasan dengan periode eksplisit, akses cepat sesuai peran/jenis usaha, grafik harian dan peringkat produk. CSS kartu sambutan lama dihapus; 19 tes UI/laporan lulus dan build berhasil. Verifikasi visual/browser belum.

### Batch 20 fungsi daftar/header restoran (B520) — testing ditunda
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
- [x] DITUNDA atas permintaan pengguna — testing B520 BELUM DIJALANKAN/BELUM LULUS; tidak menjalankan test/vet/build aplikasi/browser. Migrasi yang diminta pengguna dijalankan, seluruh versi sampai 0065 sudah applied.

Daftar /pesanan, create takeaway/delivery, edit header/kontak, erasure, cancel order dan blok edit dokumen final tersedia memakai API lama. Create tanpa meja belum idempoten server: penanda pending tab mencegah retry tak sengaja, bukan jaminan lintas tab. Detail docs/FRONTEND-BATCH-520-RESTAURANT-ORDERS.md. Integrasi luar/landing tidak disentuh.

### Migrasi lokal & stok minimum apotek (B510)
- [x] Jalankan migrate-status → migrate-up → migrate-status: 0065 shift_correction_notes diterapkan lokal, tidak ada pending pada status akhir.
- [x] Reorder mendeteksi business_type dari server; apotek memakai SUM saldo batch SELLABLE ED>tanggal lokal cabang, mengecualikan karantina/expired/disposed. Tenant/cabang/status produk/varian/cursor tetap dibatasi.
- [x] Supplier terakhir apotek berasal penerimaan batch pada cabang/varian sama; hasil memakai bentuk reorder lama dan balance_basis eksplisit.
- [x] Banner stok minimum aktif untuk apotek, link ke /apotek/kelola. Tab minimum apotek: daftar tersedia/minimum/supplier, pagination, create/edit threshold If-Match, pencarian produk track_stock, BigInt milli-unit, refresh dan konflik.
- [x] DITUNDA — testing/vet/build aplikasi/browser B510 BELUM DIJALANKAN/BELUM LULUS. Gofmt dan generator dokumentasi saja, selain migrasi yang memang diminta pengguna.

Tidak mengklaim sinkronisasi saldo inti dengan batch: hanya perhitungan reorder yang diperbaiki. Kontrak GET /stock/reorder dipindah ke implemented-not-tested karena respons apotek baru; 99 kontrak riwayat lama verified, 104 implemented-not-tested. Notifikasi provider luar tetap dilewati.

### Batch 20 fungsi riwayat/koreksi shift (B500) — testing ditunda
- [x] B500-01 Model shift_correction_notes dengan FK tenant+shift dan batas isi catatan (migrasi 0065). **Implementasi; BELUM DIUJI.**
- [x] B500-02 Trigger catatan append-only: UPDATE/DELETE/TRUNCATE ditolak, angka/status shift tidak disentuh. **Implementasi; BELUM DIUJI.**
- [x] B500-03 Indeks riwayat shift per tenant/cabang/waktu dan indeks cursor catatan. **Implementasi; BELUM DIUJI.**
- [x] B500-04 Endpoint GET /shifts: filter periode berdasarkan tanggal lokal cabang, maksimal satu tahun. **Implementasi; BELUM DIUJI.**
- [x] B500-05 Filter status all/open/closed serta terminal yang diverifikasi milik tenant/cabang. **Implementasi; BELUM DIUJI.**
- [x] B500-06 Pembatasan daftar shift sendiri bagi kasir/apoteker versus semua shift cabang berizin untuk pemilik/manajer. **Implementasi; BELUM DIUJI.**
- [x] B500-07 Pagination daftar shift dengan cursor timestamp+ID dan validasi cursor pada seluruh filter. **Implementasi; BELUM DIUJI.**
- [x] B500-08 Kas expected/count/variance shift terbuka disembunyikan (NULL) untuk melindungi hitung buta. **Implementasi; BELUM DIUJI.**
- [x] B500-09 Endpoint GET /shifts/{id}/notes: validasi tenant/cabang/pelaku dan shift harus tertutup. **Implementasi; BELUM DIUJI.**
- [x] B500-10 Pagination catatan koreksi append-only beserta pembuat dan waktu server. **Implementasi; BELUM DIUJI.**
- [x] B500-11 Endpoint POST /shifts/{id}/notes: izin shift.view_all, validasi UTF-8/trim/maksimal 1000 karakter. **Implementasi; BELUM DIUJI.**
- [x] B500-12 Idempotency wajib untuk catatan; insert catatan + audit + respons replay satu transaksi, akses diperiksa sebelum replay. **Implementasi; BELUM DIUJI.**
- [x] B500-13 Audit penambahan catatan hanya note_id tanpa body catatan agar isi bebas tidak tersebar ke audit. **Implementasi; BELUM DIUJI.**
- [x] B500-14 Halaman /shift lazy-loaded, pengenal rute aplikasi/preview, dan menu riwayat untuk peran kasir berizin. **Implementasi; BELUM DIUJI.**
- [x] B500-15 UI diisolasi per bisnis/pengguna/cabang, state pemulihan pending per cakupan. **Implementasi; BELUM DIUJI.**
- [x] B500-16 UI filter tanggal/status/terminal termasuk nonaktif dan pagination riwayat shift. **Implementasi; BELUM DIUJI.**
- [x] B500-17 Buka ringkasan shift tertutup dari sejarah meski tab/halaman sebelumnya sudah ditutup, tanpa bocor ringkasan shift terbuka. **Implementasi; BELUM DIUJI.**
- [x] B500-18 Rincian ringkasan uang/payment/refund/void/diskon memakai BigInt dan waktu zona cabang, dukungan cetak. **Implementasi; BELUM DIUJI.**
- [x] B500-19 UI baca catatan dengan pagination dan form append-only khusus pemilik/manajer, peringatan jangan masukkan rahasia/PII. **Implementasi; BELUM DIUJI.**
- [x] B500-20 Pemulihan penambahan catatan memakai snapshot/key asli B290, blok aksi baru saat belum pasti; kontrak 3 endpoint didokumentasikan implemented-not-tested. **Implementasi; BELUM DIUJI.**
- [x] DITUNDA atas permintaan pengguna — testing B500 BELUM DIJALANKAN/BELUM LULUS; tidak menjalankan test/vet/build aplikasi/browser/migrasi. Bukan bukti siap produksi.

CORE-026 catatan koreksi diimplementasikan sebagai tabel append-only terpisah, tidak menulis ulang angka/status shift tertutup. Migrasi 0065 sudah diterapkan lokal atas permintaan pengguna; status runner dikonfirmasi pada B510. UI /shift dan 3 API history/notes tersedia; kontrak implemented-not-tested. Detail docs/BATCH-500-SHIFT-HISTORY-NOTES.md. Landing dan integrasi luar tidak diubah.

### Batch 20 implementasi frontend pajak/penugasan (B490) — testing ditunda
- [x] B490-01 Halaman /pajak lazy-loaded, rute aplikasi/preview, menu dan tautan dari Pengaturan. **Implementasi; BELUM DIUJI.**
- [x] B490-02 Pembatasan UI hanya Pemilik dan reset workspace per bisnis/pengguna. **Implementasi; BELUM DIUJI.**
- [x] B490-03 Daftar aturan scoped aktif dan arsip dari backend dengan status/rate/mode. **Implementasi; BELUM DIUJI.**
- [x] B490-04 Pagination aturan cursor halaman pertama/berikutnya. **Implementasi; BELUM DIUJI.**
- [x] B490-05 Muat detail aturan terbaru sebelum membuka editor, bukan memakai snapshot list lama. **Implementasi; BELUM DIUJI.**
- [x] B490-06 Form buat aturan scoped dengan nama maksimal 100 karakter. **Implementasi; BELUM DIUJI.**
- [x] B490-07 Konversi persen tepat ke basis point string/integer terbatas 0–10000 tanpa perhitungan float tarif. **Implementasi; BELUM DIUJI.**
- [x] B490-08 Pilihan inclusive/exclusive sesuai endpoint dan validasi helper taxRuleBody. **Implementasi; BELUM DIUJI.**
- [x] B490-09 Edit aturan memakai If-Match versi aturan dan reset form menurut ID/versi. **Implementasi; BELUM DIUJI.**
- [x] B490-10 Konflik versi editor mengunci submit sampai detail terbaru dimuat ulang. **Implementasi; BELUM DIUJI.**
- [x] B490-11 Konfirmasi arsip inline tanpa window.confirm, dengan penjelasan aturan masih ditugaskan ditolak backend. **Implementasi; BELUM DIUJI.**
- [x] B490-12 Pulihkan aturan arsip memakai versi dan reload daftar. **Implementasi; BELUM DIUJI.**
- [x] B490-13 Pencarian produk/SKU untuk memilih target pajak, varian satu produk dideduplikasi. **Implementasi; BELUM DIUJI.**
- [x] B490-14 Pemilih kategori aktif sebagai target penugasan pajak. **Implementasi; BELUM DIUJI.**
- [x] B490-15 Muat penugasan target dan versi master produk/kategori dari server. **Implementasi; BELUM DIUJI.**
- [x] B490-16 Tampilkan nama aturan target saat berada di halaman aturan lain melalui lookup detail. **Implementasi; BELUM DIUJI.**
- [x] B490-17 Simpan penugasan pajak produk/kategori dengan If-Match, lepas memakai null untuk inheritance. **Implementasi; BELUM DIUJI.**
- [x] B490-18 Pilihan assignment hanya aturan aktif; aturan arsip saat ini ditandai dan tidak dipilih sebagai penugasan baru. **Implementasi; BELUM DIUJI.**
- [x] B490-19 Form pajak default memuat nilai server, mengubah nama/tarif/mode, mendukung tarif 0%, dan refresh setelah simpan. **Implementasi; BELUM DIUJI.**
- [x] B490-20 Penjelasan prioritas produk→kategori→default serta snapshot transaksi lama; state loading/error/empty/busy, konflik assignment wajib reload dan tutup editor tersedia. **Implementasi; BELUM DIUJI.**
- [x] DITUNDA atas permintaan pengguna — testing B490 BELUM DIJALANKAN/BELUM LULUS; tidak menjalankan test/vet/build aplikasi/browser. Bukan bukti siap produksi.

UI /pajak memakai API scoped/default dan assignment berversi yang sudah ada. Tidak ada perubahan backend/migrasi/landing/integrasi luar. Pilihan assignment mengikuti halaman aturan saat ini, kategori aktif belum pagination, search produk 30 hasil. Detail docs/FRONTEND-BATCH-490-TAX-SETTINGS.md. Tidak menutup task induk lain hanya demi angka subtask.

### Batch 20 implementasi frontend apotek (B480) — testing ditunda
- [x] Redesign `/apotek/kelola` mengikuti Soft UI biru: navigasi bagian dengan konteks cabang, panel filter/ekspor laporan, penataan tabel dan editor, serta layout responsif. Build dan tes logika apotek lulus; verifikasi visual/browser belum.
- [x] B480-01 Halaman `/apotek/kelola` lazy-loaded, rute aplikasi/preview dan menu apotek, tautan dari Operasi Batch. **Implementasi; BELUM DIUJI.**
- [x] B480-02 Pembatasan jenis usaha/peran dan cabang; state direset per bisnis/pengguna/cabang agar data lama tidak terbawa. **Implementasi; BELUM DIUJI.**
- [x] B480-03 Tab laporan batch, riwayat penerimaan, dan identitas obat dengan akses berdasarkan izin operasional. **Implementasi; BELUM DIUJI.**
- [x] B480-04 Laporan batch JSON di layar dari API resmi, menampilkan produk/SKU/batch/ED/status. **Implementasi; BELUM DIUJI.**
- [x] B480-05 Kontrol periode pergerakan dengan validasi tanggal kosong/urutan tanggal. **Implementasi; BELUM DIUJI.**
- [x] B480-06 Filter semua/mendekati ED/kedaluwarsa/karantina dan ambang 0–3650 hari. **Implementasi; BELUM DIUJI.**
- [x] B480-07 Penjelasan saldo/nilai saat ini versus pergerakan periode agar bukan klaim saldo historis. **Implementasi; BELUM DIUJI.**
- [x] B480-08 Kolom saldo dan pergerakan memakai BigInt milli-unit, bukan konversi float. **Implementasi; BELUM DIUJI.**
- [x] B480-09 Kolom supplier terakhir, biaya/unit dan estimasi nilai hanya dirender jika server cost_visible. **Implementasi; BELUM DIUJI.**
- [x] B480-10 Biaya yang tidak diketahui ditampilkan sebagai Belum diketahui, bukan nilai nol. **Implementasi; BELUM DIUJI.**
- [x] B480-11 Download CSV memakai periode/filter/ambang yang sama dengan laporan layar, dengan error dan busy state. **Implementasi; BELUM DIUJI.**
- [x] B480-12 Tombol refresh laporan, status loading/error/empty memakai ResourceState proyek. **Implementasi; BELUM DIUJI.**
- [x] B480-13 Riwayat penerimaan batch cabang menampilkan faktur, nomor batch, ED, status dan jumlah. **Implementasi; BELUM DIUJI.**
- [x] B480-14 Pagination cursor riwayat penerimaan dengan navigasi halaman pertama/berikutnya. **Implementasi; BELUM DIUJI.**
- [x] B480-15 Buka kartu gerakan dari laporan atau riwayat; cursor reset ketika batch berubah. **Implementasi; BELUM DIUJI.**
- [x] B480-16 Kartu gerakan berhalaman menampilkan waktu zona cabang, jenis/status, delta, saldo sesudah, alasan dan ref_id. **Implementasi; BELUM DIUJI.**
- [x] B480-17 Penelusuran penjualan/retur dari kartu batch, menampilkan status, qty dan alasan override tanpa PII pelanggan; batas 200 baris dinyatakan. **Implementasi; BELUM DIUJI.**
- [x] B480-18 Daftar identitas obat berhalaman dengan golongan/wajib resep/pembatasan khusus; pencarian produk/SKU untuk menambah identitas. **Implementasi; BELUM DIUJI.**
- [x] B480-19 Editor identitas memuat versi PRODUCT dan detail obat; record belum ada (404) membuka form kosong tanpa menelan error lain. **Implementasi; BELUM DIUJI.**
- [x] B480-20 Simpan identitas/flag resep/pembatasan memakai If-Match versi produk, proteksi konflik 409 (wajib muat ulang), label izin edar/golongan belum diverifikasi. **Implementasi; BELUM DIUJI.**
- [x] DITUNDA atas permintaan pengguna — testing B480 BELUM DIJALANKAN/BELUM LULUS; tidak menjalankan test/vet/build aplikasi/browser. Bukan bukti siap produksi.

UI laporan batch/kartu gerakan/riwayat penerimaan/editor identitas tersedia di /apotek/kelola memakai API lama. Runtime backend/skema/landing/integrasi luar tidak diubah. Detail docs/FRONTEND-BATCH-480-PHARMACY-TOOLS.md. Validasi regulasi/pilot tetap terbuka; task induk tidak ditutup otomatis hanya dari 20 perbaikan UI.

### Batch 20 perbaikan operasional frontend restoran (B470) — testing ditunda
- [x] B470-01 SSE invalidasi dan polling cadangan 5 detik di hook realtime restoran bersama. **Implementasi; belum diuji.**
- [x] B470-02 Abort request aktif dan tutup stream saat cakupan feed berubah/unmount. **Implementasi; belum diuji.**
- [x] B470-03 Jangan terapkan respons request lama ke feed bisnis/cabang/stasiun baru. **Implementasi; belum diuji.**
- [x] B470-04 Serialisasi request refresh agar polling/SSE tidak membuat request tumpang tindih. **Implementasi; belum diuji.**
- [x] B470-05 Simpan satu refresh susulan ketika invalidasi datang saat request masih berlangsung. **Implementasi; belum diuji.**
- [x] B470-06 Tangani stream open/error sebagai connected atau fallback polling. **Implementasi; belum diuji.**
- [x] B470-07 Tangani offline: tutup stream dan hentikan request polling selama offline. **Implementasi; belum diuji.**
- [x] B470-08 Hubungkan ulang stream dan refresh API saat browser kembali online. **Implementasi; belum diuji.**
- [x] B470-09 Refresh saat tab kembali visible, dengan cleanup seluruh listener/timer. **Implementasi; belum diuji.**
- [x] B470-10 Muat semua halaman tiket aktif memakai cursor; deteksi cursor berulang dan batasi 10.000 item eksplisit. **Implementasi; belum diuji.**
- [x] B470-11 Remount workspace KDS per pengguna/bisnis/cabang agar state operasional lama tidak ikut terbawa. **Implementasi; belum diuji.**
- [x] B470-12 Ganti window.prompt mundur status dengan form inline alasan wajib maksimal 200 karakter. **Implementasi; belum diuji.**
- [x] B470-13 Sesuaikan izin tombol mundur SERVED dengan izin kasir/serve, bukan hanya izin persiapan dapur. **Implementasi; belum diuji.**
- [x] B470-14 Tampilkan waktu pembaruan terakhir KDS, error feed dan tombol refresh; blok mutasi saat offline/error. **Implementasi; belum diuji.**
- [x] B470-15 Remount grid meja per pengguna/bisnis/cabang dan muat cursor meja tanpa batas lama 500 yang diam-diam terpotong. **Implementasi; belum diuji.**
- [x] B470-16 Indikator ready_count grid meja diperbarui realtime/poll, bukan snapshot sekali saat mount. **Implementasi; belum diuji.**
- [x] B470-17 Tampilkan kegagalan/truncation status order pada grid dan blok aksi meja saat feed offline/error. **Implementasi; belum diuji.**
- [x] B470-18 Pagination admin area dan meja terpisah (halaman pertama/berikutnya), reset cakupan dan pembatasan UI pemilik/manajer. **Implementasi; belum diuji.**
- [x] B470-19 Dialog cetak dapur native modal/portal mendukung Escape, focus containment dan pemulihan fokus saat ditutup. **Implementasi; belum diuji.**
- [x] B470-20 Pilih lebar kertas 58/80mm di dialog dan isolasi target cetak dari shell aplikasi; backdrop dialog tersendiri. **Implementasi; belum diuji.**
- [x] DITUNDA atas permintaan pengguna — testing B470 BELUM DIJALANKAN/BELUM LULUS. Tidak menjalankan test/vet/build aplikasi/browser. Bukan bukti siap produksi.

Implementasi realtime, pagination, izin mundur status, form alasan dan native dialog cetak; rincian docs/FRONTEND-BATCH-470-RESTAURANT-OPERATIONS.md. Feed dibatasi eksplisit 10.000 item, memakai dua stream di grid meja, belum diukur beban. Landing/integrasi luar/backend tidak diubah.

### Batch 20 implementasi kontrak transaksi/stok/akun (B460) — testing ditunda
- [x] B460-01 Implementasi kontrak `DELETE /api/v1/customers/{id}` — BELUM DIUJI.
- [x] B460-02 Implementasi kontrak `DELETE /api/v1/suppliers/{id}` — BELUM DIUJI.
- [x] B460-03 Implementasi kontrak `GET /api/v1/me` — BELUM DIUJI.
- [x] B460-04 Implementasi kontrak `GET /api/v1/memberships` — BELUM DIUJI.
- [x] B460-05 Implementasi kontrak `GET /api/v1/reports/products` — BELUM DIUJI.
- [x] B460-06 Implementasi kontrak `GET /api/v1/reports/stock` — BELUM DIUJI.
- [x] B460-07 Implementasi kontrak `GET /api/v1/reports/stock/batches` — BELUM DIUJI.
- [x] B460-08 Implementasi kontrak `GET /api/v1/sales` — BELUM DIUJI.
- [x] B460-09 Implementasi kontrak `GET /api/v1/sales/{id}` — BELUM DIUJI.
- [x] B460-10 Implementasi kontrak `GET /api/v1/stock/card` — BELUM DIUJI.
- [x] B460-11 Implementasi kontrak `GET /api/v1/workspace` — BELUM DIUJI.
- [x] B460-12 Implementasi kontrak `POST /api/v1/auth/login` — BELUM DIUJI.
- [x] B460-13 Implementasi kontrak `POST /api/v1/catalog/variant-combinations` — BELUM DIUJI.
- [x] B460-14 Implementasi kontrak `POST /api/v1/public/receipt` — BELUM DIUJI.
- [x] B460-15 Implementasi kontrak `POST /api/v1/sales/finalize` — BELUM DIUJI.
- [x] B460-16 Implementasi kontrak `POST /api/v1/sales/{id}/returns` — BELUM DIUJI.
- [x] B460-17 Implementasi kontrak `POST /api/v1/sales/{id}/void` — BELUM DIUJI.
- [x] B460-18 Implementasi kontrak `POST /api/v1/stock/adjustments` — BELUM DIUJI.
- [x] B460-19 Implementasi kontrak `PUT /api/v1/memberships/{id}` — BELUM DIUJI.
- [x] B460-20 Implementasi kontrak `PUT /api/v1/session/business` — BELUM DIUJI.
- [x] DITUNDA atas permintaan pengguna — testing B460 BELUM DIJALANKAN/BELUM LULUS. Tidak menjalankan vet/build aplikasi/browser; bukan bukti siap produksi.

Total 100 kontrak verified (riwayat lama) +100 implemented-not-tested. Kontrak transaksi/koreksi/stok/akun ditulis dari handler, tanpa perubahan runtime/frontend/landing/integrasi luar. Rincian docs/BACKEND-BATCH-460-TRANSACTION-CONTRACTS.md. Task induk OpenAPI masih terbuka, bukan dikurangi 20 karena subtask dokumentasi.

### Batch 20 kontrak laporan/ekspor/read (B450) — testing ditunda
- [x] B450-01 Implementasi kontrak `DELETE /api/v1/restaurant/category-routes/{id}` — BELUM DIUJI.
- [x] B450-02 Implementasi kontrak `DELETE /api/v1/restaurant/orders/{id}/contact` — BELUM DIUJI.
- [x] B450-03 Implementasi kontrak `GET /api/v1/audit-logs` — BELUM DIUJI.
- [x] B450-04 Implementasi kontrak `GET /api/v1/branches` — BELUM DIUJI.
- [x] B450-05 Implementasi kontrak `GET /api/v1/branches/{id}/registers` — BELUM DIUJI.
- [x] B450-06 Implementasi kontrak `GET /api/v1/reconciliation/findings` — BELUM DIUJI.
- [x] B450-07 Implementasi kontrak `GET /api/v1/report-exports` — BELUM DIUJI.
- [x] B450-08 Implementasi kontrak `GET /api/v1/report-exports/{id}` — BELUM DIUJI.
- [x] B450-09 Implementasi kontrak `GET /api/v1/reports/corrections` — BELUM DIUJI.
- [x] B450-10 Implementasi kontrak `GET /api/v1/reports/payments` — BELUM DIUJI.
- [x] B450-11 Implementasi kontrak `GET /api/v1/reports/sales` — BELUM DIUJI.
- [x] B450-12 Implementasi kontrak `GET /api/v1/restaurant/kitchen/tickets/{id}` — BELUM DIUJI.
- [x] B450-13 Implementasi kontrak `GET /api/v1/restaurant/order-status` — BELUM DIUJI.
- [x] B450-14 Implementasi kontrak `GET /api/v1/restaurant/reports/aged-orders` — BELUM DIUJI.
- [x] B450-15 Implementasi kontrak `PATCH /api/v1/restaurant/orders/{id}/lines/{line}/kitchen-status` — BELUM DIUJI.
- [x] B450-16 Implementasi kontrak `POST /api/v1/reports/corrections/exports` — BELUM DIUJI.
- [x] B450-17 Implementasi kontrak `POST /api/v1/reports/payments/exports` — BELUM DIUJI.
- [x] B450-18 Implementasi kontrak `POST /api/v1/reports/products/exports` — BELUM DIUJI.
- [x] B450-19 Implementasi kontrak `POST /api/v1/reports/sales/exports` — BELUM DIUJI.
- [x] B450-20 Implementasi kontrak `POST /api/v1/reports/stock/exports` — BELUM DIUJI.
- [x] DITUNDA atas permintaan pengguna — testing B450 BELUM DIJALANKAN/BELUM LULUS. Tidak menjalankan vet/build aplikasi/browser. Bukan bukti siap produksi.

Kontrak laporan/CSV dan read API diimplementasikan; 100 operasi riwayat lama verified +80 implemented-not-tested. Rute skeleton lainnya masih terbuka. Detail docs/BACKEND-BATCH-450-REPORT-READ-CONTRACTS.md. Tidak mengurangi task induk secara palsu dari 20 subtask dokumentasi.

### Batch 20 implementasi kontrak draft/pesanan (B440) — testing ditunda
- [x] B440-01 Implementasi kontrak `DELETE /api/v1/restaurant/orders/{id}/lines/{line}` — BELUM DIUJI.
- [x] B440-02 Implementasi kontrak `GET /api/v1/drafts` — BELUM DIUJI.
- [x] B440-03 Implementasi kontrak `GET /api/v1/restaurant/kitchen/tickets` — BELUM DIUJI.
- [x] B440-04 Implementasi kontrak `GET /api/v1/restaurant/orders` — BELUM DIUJI.
- [x] B440-05 Implementasi kontrak `GET /api/v1/restaurant/orders/{id}` — BELUM DIUJI.
- [x] B440-06 Implementasi kontrak `GET /api/v1/restaurant/orders/{id}/bill` — BELUM DIUJI.
- [x] B440-07 Implementasi kontrak `GET /api/v1/restaurant/orders/{id}/discounts` — BELUM DIUJI.
- [x] B440-08 Implementasi kontrak `GET /api/v1/restaurant/orders/{id}/lines` — BELUM DIUJI.
- [x] B440-09 Implementasi kontrak `POST /api/v1/drafts` — BELUM DIUJI.
- [x] B440-10 Implementasi kontrak `POST /api/v1/restaurant/orders` — BELUM DIUJI.
- [x] B440-11 Implementasi kontrak `POST /api/v1/restaurant/orders/{id}/cancel` — BELUM DIUJI.
- [x] B440-12 Implementasi kontrak `POST /api/v1/restaurant/orders/{id}/checkout` — BELUM DIUJI.
- [x] B440-13 Implementasi kontrak `POST /api/v1/restaurant/orders/{id}/lines` — BELUM DIUJI.
- [x] B440-14 Implementasi kontrak `POST /api/v1/restaurant/orders/{id}/lines/{line}/cancel` — BELUM DIUJI.
- [x] B440-15 Implementasi kontrak `POST /api/v1/restaurant/orders/{id}/send` — BELUM DIUJI.
- [x] B440-16 Implementasi kontrak `PUT /api/v1/drafts/{id}` — BELUM DIUJI.
- [x] B440-17 Implementasi kontrak `PUT /api/v1/restaurant/orders/{id}` — BELUM DIUJI.
- [x] B440-18 Implementasi kontrak `PUT /api/v1/restaurant/orders/{id}/discount` — BELUM DIUJI.
- [x] B440-19 Implementasi kontrak `PUT /api/v1/restaurant/orders/{id}/lines/{line}` — BELUM DIUJI.
- [x] B440-20 Implementasi kontrak `PUT /api/v1/restaurant/orders/{id}/lines/{line}/discount` — BELUM DIUJI.
- [x] DITUNDA atas permintaan pengguna — testing B440 BELUM DIJALANKAN/BELUM LULUS. Tidak menjalankan vet/build aplikasi/browser. Bukan bukti siap produksi.

Kontrak B440 diberi implemented-not-tested; total 100 verified (riwayat lama) +60 implemented-not-tested. Schema request/response dan header send/checkout idempoten, versi/cursor/stasiun, snapshot modifier, split payment; POST order 200 resume/201 baru didokumentasikan. Docs/BACKEND-BATCH-440-ORDER-CONTRACTS.md memuat rincian. Task induk OpenAPI tetap terbuka.

### Aturan tanda checklist — instruksi skip testing
`[x] DITUNDA` pada task pengujian adalah penutupan administratif atas permintaan pengguna: **BUKAN dijalankan, BUKAN lulus, BUKAN siap produksi**. Tidak ada testing/vet/build aplikasi yang dijalankan pada B430. Riwayat tes batch lama tidak diubah. Daftar penundaan ada di docs/DEFERRED-TESTING.md.

### Batch 20 implementasi kontrak pajak/akses (B430) — testing dilewati
- [x] B430-01 Implementasi kontrak `DELETE /api/v1/invitations/{id}` — belum diuji.
- [x] B430-02 Implementasi kontrak `DELETE /api/v1/sales/{id}/receipt-links` — belum diuji.
- [x] B430-03 Implementasi kontrak `DELETE /api/v1/tax-rules/{id}` — belum diuji.
- [x] B430-04 Implementasi kontrak `GET /api/v1/categories/{id}/tax-rule` — belum diuji.
- [x] B430-05 Implementasi kontrak `GET /api/v1/invitations` — belum diuji.
- [x] B430-06 Implementasi kontrak `GET /api/v1/products/{id}/tax-rule` — belum diuji.
- [x] B430-07 Implementasi kontrak `GET /api/v1/sales/{id}/returns` — belum diuji.
- [x] B430-08 Implementasi kontrak `GET /api/v1/sales/{id}/void` — belum diuji.
- [x] B430-09 Implementasi kontrak `GET /api/v1/tax-rules` — belum diuji.
- [x] B430-10 Implementasi kontrak `GET /api/v1/tax-rules/{id}` — belum diuji.
- [x] B430-11 Implementasi kontrak `POST /api/v1/auth/logout` — belum diuji.
- [x] B430-12 Implementasi kontrak `POST /api/v1/invitations` — belum diuji.
- [x] B430-13 Implementasi kontrak `POST /api/v1/invitations/accept` — belum diuji.
- [x] B430-14 Implementasi kontrak `POST /api/v1/sales/{id}/receipt-links` — belum diuji.
- [x] B430-15 Implementasi kontrak `POST /api/v1/sales/{id}/reprints` — belum diuji.
- [x] B430-16 Implementasi kontrak `POST /api/v1/tax-rules` — belum diuji.
- [x] B430-17 Implementasi kontrak `POST /api/v1/tax-rules/{id}/restore` — belum diuji.
- [x] B430-18 Implementasi kontrak `PUT /api/v1/categories/{id}/tax-rule` — belum diuji.
- [x] B430-19 Implementasi kontrak `PUT /api/v1/products/{id}/tax-rule` — belum diuji.
- [x] B430-20 Implementasi kontrak `PUT /api/v1/tax-rules/{id}` — belum diuji.

Seratus kontrak lama tetap verified; 40 kontrak B420+B430 implemented-not-tested. Operasi lainnya skeleton. Pajak, penugasan produk/kategori, undangan manual, revoke link struk, cetak ulang, histori retur/void, logout; response 204 tanpa body didokumentasikan. Tidak mengubah frontend/landing/runtime data. Tanda [x] batch hanya implementasi, bukan uji lulus.

### Batch 20 task kontrak apotek/opname/sesi (B420) — testing dilewati
- [x] B420-01 Implementasi schema `GET /api/v1/me/sessions`; belum diuji.
- [x] B420-02 Implementasi schema `GET /api/v1/pharmacy/batch-receipts`; belum diuji.
- [x] B420-03 Implementasi schema `GET /api/v1/pharmacy/batches`; belum diuji.
- [x] B420-04 Implementasi schema `GET /api/v1/pharmacy/batches/{id}/movements`; belum diuji.
- [x] B420-05 Implementasi schema `GET /api/v1/pharmacy/batches/{id}/trace`; belum diuji.
- [x] B420-06 Implementasi schema `GET /api/v1/pharmacy/fefo-preview`; belum diuji.
- [x] B420-07 Implementasi schema `GET /api/v1/pharmacy/products`; belum diuji.
- [x] B420-08 Implementasi schema `GET /api/v1/pharmacy/products/{id}`; belum diuji.
- [x] B420-09 Implementasi schema `GET /api/v1/stock/counts`; belum diuji.
- [x] B420-10 Implementasi schema `GET /api/v1/stock/counts/{id}`; belum diuji.
- [x] B420-11 Implementasi schema `POST /api/v1/me/sessions/revoke-others`; belum diuji.
- [x] B420-12 Implementasi schema `POST /api/v1/pharmacy/batch-receipts`; belum diuji.
- [x] B420-13 Implementasi schema `POST /api/v1/pharmacy/batches/{id}/count`; belum diuji.
- [x] B420-14 Implementasi schema `POST /api/v1/pharmacy/batches/{id}/dispose`; belum diuji.
- [x] B420-15 Implementasi schema `POST /api/v1/pharmacy/batches/{id}/release`; belum diuji.
- [x] B420-16 Implementasi schema `POST /api/v1/stock/counts`; belum diuji.
- [x] B420-17 Implementasi schema `POST /api/v1/stock/counts/{id}/post`; belum diuji.
- [x] B420-18 Implementasi schema `PUT /api/v1/auth/password`; belum diuji.
- [x] B420-19 Implementasi schema `PUT /api/v1/pharmacy/products/{id}`; belum diuji.
- [x] B420-20 Implementasi schema `PUT /api/v1/stock/counts/{id}/lines/{variant}`; belum diuji.

Permintaan terbaru pengguna: skip testing. Kontrak B420 ditandai implemented-not-tested, bukan verified; 100 operasi batch sebelumnya tetap verified. File tes disiapkan sebelum instruksi skip, tidak dijalankan. Gofmt dan generator dokumentasi saja, tanpa go test/vet/build aplikasi/browser. Detail docs/BACKEND-BATCH-420-PHARMACY-CONTRACTS.md.
Task induk OpenAPI tetap terbuka sampai operasi lain lengkap dan seluruh kontrak baru diuji; tanda [x] batch ini hanya berarti implementasi selesai.

### Batch 20 task kontrak operasional (B410)
- [x] B410-01 Kontrak `GET /api/v1/expenses`: schema dan tes HTTP nyata.
- [x] B410-02 Kontrak `GET /api/v1/expenses/{id}`: schema dan tes HTTP nyata.
- [x] B410-03 Kontrak `GET /api/v1/goods-receipts`: schema dan tes HTTP nyata.
- [x] B410-04 Kontrak `GET /api/v1/goods-receipts/{id}`: schema dan tes HTTP nyata.
- [x] B410-05 Kontrak `GET /api/v1/settings/operational-policy`: schema dan tes HTTP nyata.
- [x] B410-06 Kontrak `GET /api/v1/shifts/open`: schema dan tes HTTP nyata.
- [x] B410-07 Kontrak `GET /api/v1/shifts/{id}/summary`: schema dan tes HTTP nyata.
- [x] B410-08 Kontrak `GET /api/v1/stock/reorder`: schema dan tes HTTP nyata.
- [x] B410-09 Kontrak `POST /api/v1/expenses`: schema dan tes HTTP nyata.
- [x] B410-10 Kontrak `POST /api/v1/goods-receipts`: schema dan tes HTTP nyata.
- [x] B410-11 Kontrak `POST /api/v1/goods-receipts/{id}/post`: schema dan tes HTTP nyata.
- [x] B410-12 Kontrak `POST /api/v1/shifts/cash`: schema dan tes HTTP nyata.
- [x] B410-13 Kontrak `POST /api/v1/shifts/close`: schema dan tes HTTP nyata.
- [x] B410-14 Kontrak `POST /api/v1/shifts/open`: schema dan tes HTTP nyata.
- [x] B410-15 Kontrak `POST /api/v1/stock/receipts`: schema dan tes HTTP nyata.
- [x] B410-16 Kontrak `PUT /api/v1/goods-receipts/{id}`: schema dan tes HTTP nyata.
- [x] B410-17 Kontrak `PUT /api/v1/memberships/me/pin`: schema dan tes HTTP nyata.
- [x] B410-18 Kontrak `PUT /api/v1/settings/operational-policy`: schema dan tes HTTP nyata.
- [x] B410-19 Kontrak `PUT /api/v1/stock/minimums/{variant}`: schema dan tes HTTP nyata.
- [x] B410-20 Kontrak `PUT /api/v1/stock/variants/{id}/opening-cost`: schema dan tes HTTP nyata.

Total verified 80→100 operasi. Header idempotency wajib/opsional, periode/cabang, status draft/posted dan shift nullable diuji sesuai handler. Backend vet/full race/build dan parse YAML lulus. Tidak mengubah frontend, landing page, integrasi luar, atau perilaku runtime.
Task induk OpenAPI tetap terbuka; operasi lain skeleton. Rincian docs/BACKEND-BATCH-410-OPERATIONS-CONTRACTS.md.

### Batch 20 task kontrak katalog & satuan (B400)
- [x] B400-01 Kontrak `DELETE /api/v1/variants/{id}/barcodes`: schema + respons HTTP nyata.
- [x] B400-02 Kontrak `GET /api/v1/catalog/barcode`: schema + respons HTTP nyata.
- [x] B400-03 Kontrak `GET /api/v1/catalog/search`: schema + respons HTTP nyata.
- [x] B400-04 Kontrak `GET /api/v1/categories/archived`: schema + respons HTTP nyata.
- [x] B400-05 Kontrak `GET /api/v1/products/{id}`: schema + respons HTTP nyata.
- [x] B400-06 Kontrak `GET /api/v1/purchase-units/{id}`: schema + respons HTTP nyata.
- [x] B400-07 Kontrak `GET /api/v1/variants/{id}`: schema + respons HTTP nyata.
- [x] B400-08 Kontrak `GET /api/v1/variants/{id}/barcodes`: schema + respons HTTP nyata.
- [x] B400-09 Kontrak `GET /api/v1/variants/{id}/units`: schema + respons HTTP nyata.
- [x] B400-10 Kontrak `PATCH /api/v1/products/{id}/status`: schema + respons HTTP nyata.
- [x] B400-11 Kontrak `PATCH /api/v1/variants/{id}/fractional`: schema + respons HTTP nyata.
- [x] B400-12 Kontrak `PATCH /api/v1/variants/{id}/identity`: schema + respons HTTP nyata.
- [x] B400-13 Kontrak `PATCH /api/v1/variants/{id}/price`: schema + respons HTTP nyata.
- [x] B400-14 Kontrak `PATCH /api/v1/variants/{id}/status`: schema + respons HTTP nyata.
- [x] B400-15 Kontrak `POST /api/v1/products`: schema + respons HTTP nyata.
- [x] B400-16 Kontrak `POST /api/v1/purchase-units`: schema + respons HTTP nyata.
- [x] B400-17 Kontrak `POST /api/v1/purchase-units/{id}/barcodes`: schema + respons HTTP nyata.
- [x] B400-18 Kontrak `PUT /api/v1/products/{id}`: schema + respons HTTP nyata.
- [x] B400-19 Kontrak `PUT /api/v1/purchase-units/{id}`: schema + respons HTTP nyata.
- [x] B400-20 Kontrak `PUT /api/v1/purchase-units/{id}/sale-price`: schema + respons HTTP nyata.

Total kontrak verified bertambah 60→80; 20 operasi baru diuji HTTP/PostgreSQL, map atribut string dan optional/nullable diuji, If-Match harga satuan tetap opsional sesuai handler legacy. Backend vet/full race/build dan parse YAML lulus. Tidak mengubah frontend/landing/data runtime.
Task induk OpenAPI masih terbuka; operasi lain skeleton. Rincian docs/BACKEND-BATCH-400-CATALOG-CONTRACTS.md.

### Batch 20 task kontrak konfigurasi restoran (B390)
- [x] B390-01 Kontrak `DELETE /api/v1/restaurant/tables/{id}`: schema + respons HTTP nyata.
- [x] B390-02 Kontrak `GET /api/v1/restaurant/areas`: schema + respons HTTP nyata.
- [x] B390-03 Kontrak `GET /api/v1/restaurant/category-routes`: schema + respons HTTP nyata.
- [x] B390-04 Kontrak `GET /api/v1/restaurant/menu/availability`: schema + respons HTTP nyata.
- [x] B390-05 Kontrak `GET /api/v1/restaurant/products/{id}/modifiers`: schema + respons HTTP nyata.
- [x] B390-06 Kontrak `GET /api/v1/restaurant/service-charge`: schema + respons HTTP nyata.
- [x] B390-07 Kontrak `GET /api/v1/restaurant/stations`: schema + respons HTTP nyata.
- [x] B390-08 Kontrak `GET /api/v1/restaurant/tables`: schema + respons HTTP nyata.
- [x] B390-09 Kontrak `POST /api/v1/restaurant/modifier-groups`: schema + respons HTTP nyata.
- [x] B390-10 Kontrak `POST /api/v1/restaurant/modifier-groups/{id}/options`: schema + respons HTTP nyata.
- [x] B390-11 Kontrak `POST /api/v1/restaurant/stations`: schema + respons HTTP nyata.
- [x] B390-12 Kontrak `POST /api/v1/restaurant/tables/{id}/clean`: schema + respons HTTP nyata.
- [x] B390-13 Kontrak `POST /api/v1/restaurant/tables/{id}/restore`: schema + respons HTTP nyata.
- [x] B390-14 Kontrak `PUT /api/v1/restaurant/category-routes/{id}`: schema + respons HTTP nyata.
- [x] B390-15 Kontrak `PUT /api/v1/restaurant/menu/{id}/availability`: schema + respons HTTP nyata.
- [x] B390-16 Kontrak `PUT /api/v1/restaurant/modifier-groups/{id}`: schema + respons HTTP nyata.
- [x] B390-17 Kontrak `PUT /api/v1/restaurant/modifier-groups/{id}/options/{option}`: schema + respons HTTP nyata.
- [x] B390-18 Kontrak `PUT /api/v1/restaurant/service-charge`: schema + respons HTTP nyata.
- [x] B390-19 Kontrak `PUT /api/v1/restaurant/stations/{id}`: schema + respons HTTP nyata.
- [x] B390-20 Kontrak `PUT /api/v1/restaurant/tables/{id}`: schema + respons HTTP nyata.

Kontrak verified bertambah 40→60; 20 operasi baru diuji HTTP/PostgreSQL, termasuk nilai versi 0 untuk konfigurasi baru, branch query wajib, dan uang modifier string integer. Backend vet/full race/build serta parsing YAML lulus. Tidak mengubah frontend, data runtime, landing page atau integrasi luar.
Task induk OpenAPI tetap terbuka; operasi lainnya skeleton. Lihat docs/BACKEND-BATCH-390-RESTAURANT-CONTRACTS.md.

### Batch 20 task kontrak master data (B380)
- [x] B380-01 Kontrak `DELETE /api/v1/categories/{id}`: request/response + respons HTTP nyata.
- [x] B380-02 Kontrak `GET /api/v1/branches/{id}`: request/response + respons HTTP nyata.
- [x] B380-03 Kontrak `GET /api/v1/categories`: request/response + respons HTTP nyata.
- [x] B380-04 Kontrak `GET /api/v1/categories/{id}`: request/response + respons HTTP nyata.
- [x] B380-05 Kontrak `GET /api/v1/customers`: request/response + respons HTTP nyata.
- [x] B380-06 Kontrak `GET /api/v1/customers/{id}`: request/response + respons HTTP nyata.
- [x] B380-07 Kontrak `GET /api/v1/registers/{id}`: request/response + respons HTTP nyata.
- [x] B380-08 Kontrak `GET /api/v1/suppliers`: request/response + respons HTTP nyata.
- [x] B380-09 Kontrak `GET /api/v1/suppliers/{id}`: request/response + respons HTTP nyata.
- [x] B380-10 Kontrak `PATCH /api/v1/registers/{id}/status`: request/response + respons HTTP nyata.
- [x] B380-11 Kontrak `POST /api/v1/branches`: request/response + respons HTTP nyata.
- [x] B380-12 Kontrak `POST /api/v1/branches/{id}/registers`: request/response + respons HTTP nyata.
- [x] B380-13 Kontrak `POST /api/v1/categories`: request/response + respons HTTP nyata.
- [x] B380-14 Kontrak `POST /api/v1/categories/{id}/restore`: request/response + respons HTTP nyata.
- [x] B380-15 Kontrak `POST /api/v1/customers`: request/response + respons HTTP nyata.
- [x] B380-16 Kontrak `POST /api/v1/suppliers`: request/response + respons HTTP nyata.
- [x] B380-17 Kontrak `PUT /api/v1/branches/{id}`: request/response + respons HTTP nyata.
- [x] B380-18 Kontrak `PUT /api/v1/categories/{id}`: request/response + respons HTTP nyata.
- [x] B380-19 Kontrak `PUT /api/v1/customers/{id}`: request/response + respons HTTP nyata.
- [x] B380-20 Kontrak `PUT /api/v1/suppliers/{id}`: request/response + respons HTTP nyata.

Total verified bertambah 20→40 operasi, tanpa menambah fitur runtime atau menyentuh frontend/landing page. Query daftar pelanggan/supplier q/after didokumentasikan. Tes HTTP/PostgreSQL untuk seluruh batch lama+baru, YAML, backend vet/full race/build lulus. Rincian docs/BACKEND-BATCH-380-MASTER-CONTRACTS.md.
Task induk OpenAPI tetap terbuka; operasi lain masih skeleton. Validator schema tes hanya subset yang digunakan, bukan validator OpenAPI umum.

### Batch 20 task kontrak API (B370)
- [x] B370-01 Kontrak `DELETE /api/v1/onboarding/demo`: schema + tes HTTP nyata.
- [x] B370-02 Kontrak `DELETE /api/v1/restaurant/areas/{id}`: schema + tes HTTP nyata.
- [x] B370-03 Kontrak `GET /api/v1/me/profile`: schema + tes HTTP nyata.
- [x] B370-04 Kontrak `GET /api/v1/onboarding`: schema + tes HTTP nyata.
- [x] B370-05 Kontrak `GET /api/v1/settings/business-profile`: schema + tes HTTP nyata.
- [x] B370-06 Kontrak `GET /api/v1/settings/business-type`: schema + tes HTTP nyata.
- [x] B370-07 Kontrak `GET /api/v1/settings/tax`: schema + tes HTTP nyata.
- [x] B370-08 Kontrak `GET /api/v1/variants/{id}/unit-defaults`: schema + tes HTTP nyata.
- [x] B370-09 Kontrak `POST /api/v1/onboarding/demo`: schema + tes HTTP nyata.
- [x] B370-10 Kontrak `POST /api/v1/onboarding/events`: schema + tes HTTP nyata.
- [x] B370-11 Kontrak `POST /api/v1/restaurant/areas`: schema + tes HTTP nyata.
- [x] B370-12 Kontrak `POST /api/v1/restaurant/areas/{id}/restore`: schema + tes HTTP nyata.
- [x] B370-13 Kontrak `POST /api/v1/restaurant/tables`: schema + tes HTTP nyata.
- [x] B370-14 Kontrak `PUT /api/v1/me/profile`: schema + tes HTTP nyata.
- [x] B370-15 Kontrak `PUT /api/v1/onboarding/dismiss`: schema + tes HTTP nyata.
- [x] B370-16 Kontrak `PUT /api/v1/restaurant/areas/{id}`: schema + tes HTTP nyata.
- [x] B370-17 Kontrak `PUT /api/v1/settings/business-profile`: schema + tes HTTP nyata.
- [x] B370-18 Kontrak `PUT /api/v1/settings/business-type`: schema + tes HTTP nyata.
- [x] B370-19 Kontrak `PUT /api/v1/settings/tax`: schema + tes HTTP nyata.
- [x] B370-20 Kontrak `PUT /api/v1/variants/{id}/unit-defaults`: schema + tes HTTP nyata.

Verifikasi: schema request/response dan required/nullability/status/header dikurasi dari handler; error envelope sama. 20 operasi diberi x-contract-status verified, sisanya skeleton. Tes HTTP semua 20 operasi, drift rute, operationId, dan negatif validator lulus. Backend vet/full race/build dan parse YAML lulus. Lihat docs/BACKEND-BATCH-370-OPENAPI.md.
Task induk OpenAPI tetap terbuka karena operasi lain belum punya schema; tidak mengurangi checklist induk sebesar 20. Tidak ada perubahan frontend/landing/data runtime.

### Rekonsiliasi ongkir laporan & retur (B360)
- [x] Migrasi 0064: sale_returns.delivery_fee_cents menyimpan ongkir direfund (nol parsial, fee sale saat retur penuh); historis dengan ongkir tidak diketahui tetap NULL, bukan direkonstruksi.
- [x] Laporan penjualan JSON/CSV sinkron dan ekspor latar menambah delivery_fee_cents dan net_bill_cents (neto+pajak+SC+ongkir+pembulatan); frontend menampilkan SC/ongkir/pembulatan/tagihan neto.
- [x] Tes DB membandingkan laporan penjualan dengan pembayaran neto sebelum retur/parsial/penuh; snapshot refund fee diverifikasi dan histori NULL menandai laporan incomplete. Tes ekspor CSV ongkir/tagihan ditambahkan.
- [x] Backend vet/full race/build, frontend 111 tes/build lulus, migrasi lokal diterapkan. Tidak mengubah landing page/integrasi luar.
- [ ] Batas: retur historis berongkir tanpa snapshot akan tetap incomplete dan memerlukan rekonsiliasi manual. Uji browser nyata belum. Kebijakan ongkir hanya direfund saat retur penuh tetap [REK].

### Pengukuran performa & struk snapshot (B350)
- [x] Benchmark opt-in `STOREPOS_PERF_CHECK=1`: pencarian HTTP 2 tenant ×50.000 varian dan finalisasi HTTP satu item berstok. Filter nama/SKU/barcode/no-result/literal wildcard dicek; saldo dan unique struk finalisasi diverifikasi.
- [x] Query katalog mengganti EXISTS berkorelasi barcode dengan kandidat IN tenant+barcode sekali. Sebelum p95 teks/barcode 265–283ms, sesudah 167–183ms; ulang -race p95 26–189ms (empty query lebih cepat).
- [x] Finalisasi HTTP lokal p95 4,23ms biasa dan 5,55ms -race (30 sampel sesudah warmup). Bukan klaim produksi atau multi-item concurrent load.
- [x] ReceiptPaper merender modifier/catatan snapshot dengan escaping React; tes render teks tak aman ditambahkan. Frontend 111 tes/build, backend vet/full race/build, benchmark opt-in -race lulus.
- [ ] Belum: load produksi/perangkat kasir/jaringan nyata; benchmark opt-in default skip agar CI tidak tergantung timing hardware. Tidak menambah indeks/dependency tanpa kebutuhan terukur.

### Pelanggan & peringatan stok (B340)
- [x] `GET /sales` menerima customer_id tervalidasi tenant; query data dan cursor memakai filter yang sama. Tanpa filter tetap kompatibel.
- [x] Daftar pelanggan: cari nama/telepon, edit kontak, tautan histori ke `/transaksi?customer_id=...`; halaman transaksi mengisolasi state per bisnis/cabang/pelanggan.
- [x] `LowStockNotice` di shell aplikasi memeriksa reorder saat mount dan tiap 60 detik, memberi peringatan+tautan pengadaan; tidak memaksakan saldo inti untuk apotek.
- [x] Tes API histori: hanya transaksi pelanggan terpilih, foreign customer 404, ID rusak 422, cursor pelanggan lain 422, tanpa filter kompatibel. Backend vet/full race/build lulus; frontend 110 tes/build lulus.
- [ ] Belum: browser nyata; peringatan minimum apotek belum (stok batch terpisah), legal/PDP dan notifikasi provider eksternal sengaja dilewati.

### Dokumen penerimaan & pemulihan (B330)
- [x] `/penerimaan`: daftar dengan cursor/periode/status/faktur, buka detail draft/posted, edit supplier/tanggal/faktur/satuan/jumlah/harga/hapus baris, simpan If-Match, muat ulang status server.
- [x] Posting harus setelah simpan data terbaru, snapshot key/payload/versi di sessionStorage per bisnis/pengguna/cabang memakai B290; tombol posting lama di Operasi Stok juga memakai mekanisme sama.
- [x] Backend create receipt mendukung Idempotency-Key opsional untuk kompatibilitas klien lama; UI bawaan selalu mengirim key. Replay ID sama, konflik payload ditolak; detail menyertakan variant_id agar satuan dipetakan tepat.
- [x] Tes backend diperluas untuk replay create/conflict/detail variant_id; helper frontend roundtrip uang besar/qty pecahan tanpa float dan validasi baris. Backend vet/full race/build lulus; frontend 110 tes/build lulus.
- [ ] Batas: belum uji browser/disconnect nyata; key masih opsional bagi klien lama; halaman edit tidak menambah produk baru ke draft (bisa ubah/hapus baris yang sudah ada), penambahan multi-produk memakai form pembuatan di Operasi Stok.

### Satuan default & ringkasan shift (B320)
- [x] Katalog `ProductUnits`: atur default beli/jual (null = satuan dasar), If-Match dan refresh konflik, opsi jual hanya yang punya harga; penerimaan dan tombol tambah kasir memakai default.
- [x] Scan barcode satuan/dasar dan pilihan satuan eksplisit tidak ditimpa default. Default hilang/arsip ditolak, bukan diam-diam berganti ke satuan dasar; helper `units.js` + 2 tes. Request tambah kasir dikunci selama lookup/simpan.
- [x] CloseShift mengembalikan shift_id; UI memuat ringkasan pembayaran, refund tunai/void, kas masuk/keluar, diskon dan cetak dari summary server. Tes API ID close->summary ditambahkan.
- [x] Backend vet/full race/build lulus; frontend 108 tes/build lulus (sebelum penguncian klik terakhir, tes/build diulang setelahnya).
- [ ] Belum: uji browser nyata. Banyak endpoint konfigurasi memakai pola reload saat konflik; tidak mengubah histori/snapshot transaksi.

### Wizard konfigurasi awal (B310)
- [x] `/mulai`: profil bisnis, jenis usaha (terkunci setelah sale), buat/edit cabang dan terminal awal, pajak/pembulatan/struk, service charge per jenis order restoran, produk pertama; tautan dari checklist dashboard.
- [x] Simpan langkah dan lanjut dipisahkan agar retry event tidak menulis ulang konfigurasi. Kegagalan simpan mengunci form sampai nilai/versi terbaru dimuat ulang. Langkah boleh dilewati.
- [x] Backend milestone onboarding ke audit append-only, tanpa payload/PII, whitelist langkah/status dan deduplikasi dengan lock bisnis; test izin/isolation/deduplikasi lulus, OpenAPI diperbarui.
- [x] Rute aplikasi `/mulai` dan `/menu-habis` dikenali website/preview router. Frontend 106 tes/build lulus; backend vet/full race/build lulus.
- [ ] Batas: wizard untuk bisnis yang sudah dimiliki, bukan registrasi/business-creation; pengaturan multi-endpoint tidak atomik dan harus dimuat ulang jika sebagian gagal. Event completed/skipped menyatakan navigasi pengguna, bukan bukti konfigurasi valid; checklist dan M2 tetap diturunkan dari data server. Browser nyata belum diuji.

### Penyelesaian UI pesanan restoran (B300)
- [x] Checkout restoran menerima beberapa baris pembayaran tunai/QRIS/kartu/transfer manual; setiap baris wajib nominal positif, payload finansial tetap memakai snapshot/key retry B290.
- [x] Form diskon pesanan dan per baris memakai endpoint server dengan If-Match; jenis none/persen/nominal memakai helper diskon kasir yang sama, tagihan menampilkan diskon baris/transaksi.
- [x] Edit qty dan catatan item NOT_SENT via endpoint PUT; konversi milli-unit tepat tanpa float dan snapshot modifier tetap dipertahankan.
- [x] Tiga tes helper ditambahkan: split pembayaran, edit item, diskon; frontend 105 tes lulus dan build berhasil. Tidak ada perubahan backend/skema/landing page.
- [ ] Belum: persetujuan PIN diskon checkout. Tidak menyimpan PIN di pending snapshot browser; alur diskon di atas batas kasir tetap ditolak server. Uji browser nyata belum.

### Pemulihan mutasi finansial/stok (B290)
- [x] Checkout dan kirim dapur restoran serta penerimaan/pelepasan/pemusnahan/opname batch menyimpan snapshot path/payload/version/Idempotency-Key sebelum request; retry tidak membaca form atau versi terbaru.
- [x] Pending disimpan di sessionStorage per pengguna/bisnis/order atau cabang, dipulihkan setelah refresh, dan aksi baru diblokir sampai hasil pasti. Kredensial tidak dimasukkan ke snapshot.
- [x] Timeout/network/5xx/408/429/IDEMPOTENCY_CONFLICT mempertahankan pending; validasi pasti menghapus pending. Penyimpanan gagal atau data rusak menghentikan pengiriman. Replay lewat 72 jam diblokir karena retensi key backend.
- [x] Enam tes helper mencakup timeout/retry identik, perubahan form, scope pengguna, respons pasti/tidak pasti, storage gagal, payload rusak, masa retensi, dan kirim dapur tanpa body. Frontend 102 tes lulus; build berhasil.
- [ ] Belum: uji browser disconnect/refresh nyata. sessionStorage hanya bertahan di tab yang sama; tab ditutup atau storage dibersihkan memerlukan pemeriksaan status server sebelum mengulangi aksi. Integrasi luar tetap dilewati.

### Hardening rantai middleware (B280)
- [x] Tes urutan CSRF → auth → tenant → modul → izin → cabang; setiap kegagalan menghentikan lapisan berikutnya, jalur sukses mencapai handler.
- [x] Logging 403 dipusatkan di `httpx.WriteError`; log izin lama yang memuat path mentah dihapus agar tidak duplikat atau membocorkan ID.
- [x] Tes log aman memastikan tidak mencatat path mentah, token query, payload, atau pesan internal. `go vet` dan seluruh suite backend `-race` lulus.
- [ ] Tidak diubah: access log umum memiliki kebijakan terpisah; ini hanya hardening log penolakan akses. Provider observability eksternal tetap dilewati.

### Metrik & module guard (B270)
- [x] Middleware metrik in-process: request/5xx + histogram latency per method dan route pattern (tanpa UUID/tenant/user), endpoint Prometheus text `/internal/metrics` loopback-only, tanpa dependency/provider eksternal
- [x] Tes metrik: agregasi, 5xx, histogram, cardinality, loopback, endpoint tidak mengukur diri sendiri
- [x] `module_guard_test.go`: 63 rute restoran/apotek menolak bisnis retail dengan 403 MODULE_DISABLED; halaman modul restoran/apotek lazy-loaded
- [ ] Belum: dashboard/alert provider eksternal (disengaja skip), persistence metrik hilang saat proses restart

### UI operasional apotek & dapur (B260)
- [x] `/batch`: form inline pelepasan karantina/pemusnahan (menghapus `window.prompt`), opname per batch/status, penelusuran penerimaan->penjualan/retur, unduh laporan CSV periode/filter
- [x] Grid meja menampilkan `ready_count` sebagai indikator item siap diantar; KDS punya pratinjau/cetak tiket detail snapshot via endpoint server dan CSS 80 mm
- [x] Helper apotek + 3 tes baru; frontend `npm test` 96 lulus dan build berhasil
- [ ] Belum: uji browser/printer fisik; print CSS 58 mm bergantung scaling driver browser; trace dibatasi 200 baris per bagian tanpa pagination

### Pengaturan akun & master restoran (B250)
- [x] UI ganti kata sandi di Pengaturan: kata sandi lama, kebijakan minimal/konfirmasi, backend mencabut sesi perangkat lain; `password.js` + 2 tes
- [x] UI `/meja/kelola`: buat/edit/arsip/pulihkan area dan meja, version/409 reload, hanya Pemilik/Manajer; menu navigasi ditambahkan
- [x] Tes backend lifecycle area/meja: buat/edit, blok arsip area dengan meja aktif, arsip/pulihkan, blok meja dengan order terbuka; suite backend `-race` lulus
- [x] Frontend `npm test` 93 lulus dan build berhasil
- [ ] Belum: uji browser nyata; halaman daftar area/meja memuat maksimal 100 baris (pagination admin belum)

### Lifecycle master data (B240)
- [x] `docs/MASTER-DATA-LIFECYCLE.md`: keputusan delete/archive/anonymize/revoke untuk produk, varian, satuan, kategori, supplier, pelanggan, cabang, terminal, anggota, pajak, area/meja, barcode, undangan, link struk, kontak/baris order, dan data contoh
- [x] `master_lifecycle_test.go`: semua rute DELETE wajib masuk allowlist dengan kebijakan eksplisit; master/dokumen historis utama dilarang punya DELETE permanen
- [x] Tes menangkap dan perbaiki bug supplier batch apotek (500 FK -> 409); suite backend `-race` lulus
- [ ] Di luar cakupan: penghapusan seluruh tenant mengikuti retensi/legal; hak SQL deployment (`REVOKE`) tetap lapisan terpisah

### Kebijakan hapus vs arsip (B230)
- [x] Bug ditemukan tes: hapus supplier yang dipakai penerimaan batch apotek menghasilkan 500 dari FK database; kini 409 (arsipkan). Ditambal di `deleteSupplier`
- [x] Tes: `supplier_delete_test.go` (supplier tak terpakai boleh dihapus, terpakai 409) dan produk/varian tanpa rute DELETE; suite backend `-race` lulus, `docs/openapi.yaml` diperbarui
- [ ] Belum: audit sistematis semua master data terhadap FK (satuan, terminal, cabang, pengguna, area/meja); tes hanya mencakup supplier, produk, dan varian

### Laporan stok per batch (B220)
- [x] `reporting.BatchStock` + `GET /reports/stock/batches`; CSV memakai `csvText` (anti formula injection) dan menulis audit `report.exported`
- [x] Tes: filter, EXPIRED turunan, biaya tertimbang (2@1000 + 8@2000 = 1800), batch tanpa harga beli bernilai kosong bukan 0, biaya tidak bocor ke peran tanpa izin, periode lampau; suite `-race` lulus
- [ ] Belum: ekspor lewat antrean latar, estimasi biaya memakai harga beli penerimaan batch (bukan HPP inti, karena saldo batch terpisah dari stok inti)

### Onboarding (B210)
- [x] Backend `GET /onboarding` (progres 5 langkah dari data nyata, produk contoh tidak dihitung), `PUT /onboarding/dismiss`, `POST/DELETE /onboarding/demo`; migrasi 0063 (`products.is_demo`)
- [x] Produk contoh tanpa pelacakan stok (tidak menyentuh kartu stok append-only), bernama "[Contoh] ...", SKU berawalan `DEMO-`; apotek hanya barang non-obat
- [x] Frontend: panel checklist di dashboard (progressbar beraria, tautan ke langkah berikutnya), tombol tambah/hapus produk contoh; `onboarding.js` + 5 tes, `npm test` 91 lulus, build berhasil
- [x] Tes backend: demo idempoten dan terisolasi antar bisnis, produk pengguna tidak tersentuh, checklist diturunkan dari data; suite `-race` lulus, `docs/openapi.yaml` dibangkitkan ulang
- [ ] Belum: wizard satu alur (bisnis, jenis usaha, cabang, pajak, produk) — saat ini pengaturan tersebar di halaman Pengaturan; event onboarding untuk M2 — M2 hanya dihitung dari selisih `created_at` dan penjualan pertama, belum ada pencatatan event per langkah

### UI pembatalan item dan menu habis (B200)
- [x] Halaman pesanan: tombol Batalkan untuk item yang sudah dikirim, form inline (bukan `window.prompt`) dengan alasan wajib dan email+PIN atasan bila status/peran mewajibkan
- [x] Halaman `/menu-habis`: daftar menu dengan stok tersedia (saldo dikurangi cadangan dapur), tombol Tandai habis/Tersedia kembali dengan versi; 409 memuat ulang
- [x] Logika murni di `orders.js` + 4 tes baru; `npm test` 86 lulus, build berhasil
- [x] Tes backend batas ED apotek: ED = hari ini ditolak, ED besok lolos, tanggal lokal cabang (UTC+14 vs UTC-11) menentukan
- [ ] Belum: uji browser nyata untuk semua halaman baru; aturan "perlu persetujuan" di UI hanya petunjuk, server yang memutuskan

### Kerangka OpenAPI (B190)
- [x] `docs/openapi.yaml` dibangkitkan dari rute terdaftar: metode, path, parameter path, izin (`x-permission`), autentikasi sesi, header bisnis/CSRF/Idempotency-Key/If-Match; YAML valid
- [x] Tes `TestOpenAPIMatchesRegisteredRoutes` gagal bila berkas tertinggal dari rute; `TestOperationIDsAreUnique` menjaga operationId unik. Perbarui dengan `UPDATE_OPENAPI=1 go test ./internal/api -run TestOpenAPI`
- [ ] Belum: skema body dan respons, contoh, kode error per endpoint, dan validasi respons nyata terhadap spesifikasi

### State machine status (B180)
- [x] Trigger `enforce_status_transition` (migrasi 0062): sales completed->voided saja; payments confirmed->reversed saja; pesanan restoran open->completed/cancelled; draf active/held->active/held/cancelled/completed; status retur hanya maju none->partial->full
- [x] SQLSTATE `ST001` dipetakan `httpx.WriteError` menjadi 409 `INVALID_STATE_TRANSITION`; error database lain tetap 500 tanpa membocorkan pesan
- [x] Tes: `state_machine_test.go` (transisi sah lolos, tidak sah ditolak lewat SQL langsung) dan pemetaan error; suite backend `-race` lulus
- [ ] Belum: transisi baris pesanan dapur (NOT_SENT..SERVED) dan status shift/laporan masih diatur di handler, bukan trigger

### Hardening backend (B170)
- [x] Tes `route_matrix_test.go`: 197 rute non-publik dibaca dari kode dan harus menolak request tanpa sesi (401/403); rute publik hanya lewat daftar putih (login, struk publik, leads)
- [x] Tes audit append-only: UPDATE, DELETE, dan TRUNCATE ditolak database. TRUNCATE sebelumnya lolos dan bisa mengosongkan audit; ditutup di migrasi 0061 untuk audit_logs, stock_movements, pharmacy_batch_movements
- [x] Matriks peran (`role_matrix_test.go`) dan isolasi tenant (header bisnis lain + ID lintas bisnis) lulus
- [ ] Belum: `REVOKE` pada role aplikasi (role belum dibuat), RLS, OpenAPI. 6 rute dibangun dari perulangan string ditulis manual di tes; tes gagal bila jumlah pendaftaran dinamis berubah

### Ongkir, kata sandi, sesi (B160)
- [x] Ongkir restoran (DELIVERY_MANUAL) ikut total transaksi: tidak kena pajak/diskon/service charge, masuk pembulatan tunai, tersimpan di `sales.delivery_fee_cents` (migrasi 0059)
- [x] Retur penuh mengembalikan ongkir bersama pembulatan, sehingga total refund = total dibayar; retur parsial tidak mengembalikan ongkir [REK]
- [x] Struk publik, detail transaksi, dan tagihan pesanan menampilkan ongkir; checkout restoran tidak lagi menolak ongkir nonzero
- [x] Kata sandi umum ditolak (`VerifyPolicy`); `PUT /auth/password`: kata sandi lama wajib benar, batas percobaan, sesi lain dicabut, audit tanpa rahasia
- [x] Tes: 4 tes ongkir (total, pembulatan, batas, retur penuh), 1 tes ganti kata sandi, 1 tes akun nonaktif; suite backend `-race` lulus
- [ ] Belum: daftar kata sandi umum yang lengkap, keputusan [REK] ongkir pada retur parsial. UI ganti kata sandi selesai B250; laporan ongkir terpisah dan tagihan neto selesai B360.

### Frontend pesanan restoran (B150)
- [x] Halaman `/pesanan/:id`: cari menu, pilih modifier (validasi min/maks grup), jumlah, catatan; tambah dan hapus item belum dikirim
- [x] Kirim ke dapur (key idempoten per klik), tagihan dari server, alasan pembayaran diblokir (item belum dikirim)
- [x] Bayar satu metode dengan key idempoten yang dipakai ulang saat status tidak pasti; 409 memuat ulang tampilan
- [x] Grid meja: tombol "Buka pesanan"/"Lihat pesanan" menyambung meja ke pesanan (server melanjutkan pesanan terbuka)
- [x] Backend: hasil `/catalog/search` kini membawa `product_id` (untuk memuat modifier)
- [x] Logika murni `orders.js` + 6 tes; `npm test` 77 lulus, build dan `go vet` bersih, suite backend `-race` lulus
- [ ] Belum: persetujuan PIN diskon saat checkout, pindah meja, uji browser nyata. Split pembayaran/diskon pesanan dan baris/edit jumlah-catatan selesai di B300; pembatalan item terkirim tersedia sejak B200.

### Frontend grid meja (B140)
- [x] Redesign Soft UI biru: ringkasan status interaktif, pencarian meja/area, filter area, kartu kapasitas/status dan item siap diantar, aksi lebar penuh, layout responsif; tes filter dan build lulus
- [x] Halaman `/meja`: meja dikelompokkan per area, status turunan server (Kosong/Terisi/Perlu dibersihkan) berupa teks + warna, ringkasan jumlah, polling 5 detik
- [x] Aksi "Tandai bersih" hanya untuk meja perlu dibersihkan, memakai versi meja; 409 memuat ulang tampilan
- [x] Logika murni `tables.js` + 5 tes; `npm test` 71 lulus, build berhasil
- [x] Bentuk `modifiers` KDS diverifikasi ke backend (`ModifierSnapshot.name`), tebakan sebelumnya benar
- [ ] Belum: buka pesanan dari meja (UI pesanan restoran), pindah meja, uji browser nyata, E2E

### Frontend layar dapur (B130)
- [x] Halaman `/dapur` (KDS): tiket per stasiun, teks besar, aksi satu ketuk maju satu langkah sesuai peran (dapur: Mulai masak/Tandai siap, kasir: Sajikan)
- [x] Penyegaran UI dapur: ringkasan stasiun/tiket, filter Semua/Baru/Dimasak/Siap dengan hitungan baris menu, kartu responsif dengan jenis pesanan/nomor tiket/umur/status/catatan dan aksi lebar. Data stasiun lama disembunyikan ketika berganti; state memuat/kosong/koneksi tersedia. Build dan uji helper KDS lulus; visual browser tetap perlu pemeriksaan
- [x] Umur tiket dari waktu server (tahan jam perangkat melenceng), nada 10/20 menit, tanda "Siap diantar", item dibatalkan tampil dicoret
- [x] Banner koneksi terputus, SSE sebagai sinyal refresh + polling 5 detik sebagai cadangan, 409 memuat ulang tampilan
- [x] Mundur status meminta alasan (sesuai backend); logika murni `kds.js` + 8 tes, `npm test` 66 lulus, build berhasil
- [ ] Belum: uji browser nyata, uji jam sibuk >= 30 tiket/jam, dialog pengganti `window.prompt`, suara/notifikasi tiket baru, E2E alur dapur

### Refund split tender (B120)
- [x] Backend: refund retur dibagi ke metode pembayaran asal (nontunai dulu, tunai terakhir, dibatasi sisa per metode), satu baris `refunds` per metode, shift hanya untuk bagian tunai
- [x] Retur apotek tidak lagi diblokir (masuk karantina batch asal, lihat B100)
- [x] Frontend: pembatas "retur split belum didukung" dihapus, rincian refund per metode (`refundSummary`)
- [x] Tes: alokasi refund, retur split dua metode, retur kedua tidak melebihi total bayar; `npm test` 58 lulus, suite backend `-race` lulus
- [ ] Keputusan produk [REK]: urutan nontunai dulu lalu tunai belum divalidasi; bagian nontunai tetap manual (belum ada integrasi pembayaran); belum ada tes HTTP dan tes konkurensi retur paralel

### Frontend apotek (B110)
- [x] Halaman `/batch` (Batch & ED): daftar stok per batch, filter mendekati ED/kedaluwarsa/karantina, label ED, terima batch, lepas karantina (apoteker), musnahkan
- [x] Checkout kasir apotek: referensi resep + verifikasi apoteker (email+PIN) dikirim ke finalisasi
- [x] Logika murni `pharmacy.js` + 5 tes; `npm test` 55 lulus, `npm run build` berhasil
- [x] Pratinjau FEFO + label ED per baris keranjang (`FefoPreview`) dan pesan khusus BATCH_EXPIRED/PRESCRIPTION_REFERENCE_REQUIRED; `npm test` 57 lulus
- [ ] Belum: tampilan penelusuran batch, UI opname per batch, dialog pengganti `window.prompt`, uji browser nyata

### Batch backend B100 — finalisasi & siklus batch apotek (implementasi)
- [x] B100-01 Finalisasi apotek tidak lagi diblokir: alokasi FEFO dikunci di transaksi (urut varian/ED/ID) (APT-005)
- [x] B100-02 Snapshot batch/ED per alokasi pada `sale_line_batches` (APT-005/011)
- [x] B100-03 Satu baris terbagi ke beberapa batch; kurang stok → STOCK_INSUFFICIENT (APT-005)
- [x] B100-04 Batch kedaluwarsa tidak dapat dijual, tanggal lokal cabang, BATCH_EXPIRED (APT-006)
- [x] B100-05 Referensi resep + PRESCRIPTION_REFERENCE_REQUIRED untuk produk wajib resep (APT-008)
- [x] B100-06 Verifikasi apoteker lewat login atau email+PIN apoteker (APT-009)
- [x] B100-07 Override batch manual hanya apoteker + alasan (APT-005)
- [x] B100-08 Void mengembalikan stok ke batch asal (APT-011)
- [x] B100-09 Retur restock masuk karantina batch asal, rusak hanya menutup alokasi (APT-010)
- [x] B100-10 Penelusuran batch penerimaan → penjualan → retur tanpa PII pelanggan (APT-011)
- [x] B100-11 Opname per batch/status dengan COUNT_ADJUSTMENT, alasan, key, audit (APT-013)

Migrasi 0057–0058, build/vet/suite lama -race lulus. Tes backend apotek ditambahkan (`sales/pharmacy_test.go`): FEFO lintas batch, ED/karantina tidak terjual, resep+apoteker, override batch, 6 penjualan paralel tanpa oversell, void dan retur ke karantina. Belum ada tes HTTP/endpoint dan tes opname. Saldo batch terpisah dari `inventory_balances` inti (stok inti apotek tidak dikurangi saat jual), jadi laporan stok/HPP inti belum mewakili apotek. Refund split tender masih diblokir. Bukan klaim kepatuhan regulasi.

### Batch backend 20 task — fondasi apotek (implementasi awal)
- [x] B90-01 Simpan identitas obat produk dengan versi produk (APT-001)
- [x] B90-02 Detail identitas obat tenant (APT-001)
- [x] B90-03 Daftar obat/flag resep/pembatasan dengan cursor (APT-001)
- [x] B90-04 Audit perubahan wajib resep/pembatasan tanpa penentuan golongan otomatis (APT-001)
- [x] B90-05 Pembatasan modul/izin/cabang endpoint apotek (CORE-002/005)
- [x] B90-06 Model batch unik tenant/varian/nomor/ED (APT-003)
- [x] B90-07 Parser ED tanggal atau bulan→akhir bulan (APT-003)
- [x] B90-08 Tanggal lokal cabang untuk batas kedaluwarsa (APT-006)
- [x] B90-09 Penerimaan batch wajib supplier/faktur/qty/HPP/status (APT-003)
- [x] B90-10 ED <=hari ini hanya dapat diterima karantina (APT-003)
- [x] B90-11 Penerimaan batch sama menambah saldo, setiap dokumen tersimpan (APT-003)
- [x] B90-12 Idempotency/audit penerimaan batch atomik (APT-003/CORE-017)
- [x] B90-13 Saldo per cabang/batch/status nonnegatif (APT-004)
- [x] B90-14 Pergerakan batch append-only dan balance_after (APT-011)
- [x] B90-15 Daftar stok batch/status EXPIRED turunan (APT-004)
- [x] B90-16 Filter near_expiry/expired/quarantine, supplier terakhir dan days-to-expiry (APT-007)
- [x] B90-17 Pratinjau FEFO sellable/nonexpired dan shortage, tanpa finalisasi (APT-005)
- [x] B90-18 Pelepasan karantina hanya Apoteker dan ED valid, reason/key/audit (APT-010)
- [x] B90-19 Dispose stok batch dengan alasan/key/audit tanpa saldo negatif (APT-012)
- [x] B90-20 Riwayat penerimaan/kartu batch + blok finalisasi generic apotek sampai FEFO/resep atomik siap (APT-011/006)

Migrasi 0056 diterapkan; build/vet/suite lama -race lulus. Tidak ada fungsi tes baru; alur apotek baru belum diuji khusus. Batch balances terpisah dari inventory_balances/HPP core, belum integrasi costing/report menyeluruh. Daftar batch dibatasi 101 dan truncated eksplisit, belum compound cursor status. Finalisasi/resep/retur/opname batch tetap terbuka, jangan dipakai pilot produksi. Frontend tidak diubah.

### Batch backend 10 task — impor CSV (implementasi)
- [x] B80-01 Template CSV katalog dengan schema kolom stabil (CORE-012)
- [x] B80-02 Parser encoding/csv UTF-8/BOM/header dan batas 5 MiB/5.000 baris (CORE-012)
- [x] B80-03 Validasi nama/SKU/satuan/harga/bool/qty/HPP dengan error baris/field (CORE-012)
- [x] B80-04 Deteksi SKU case-insensitive/barcode duplikat file dan tenant (CORE-008/012)
- [x] B80-05 Endpoint preview valid/errors tanpa perubahan stok/katalog (CORE-012)
- [x] B80-06 Commit seluruh file dalam transaksi atomik, konflik bersamaan rollback (CORE-012)
- [x] B80-07 Stok awal OPENING_BALANCE append-only dalam satuan dasar (CORE-012/032)
- [x] B80-08 HPP stok awal positif wajib dan disimpan pada balance (CORE-012/§13.11)
- [x] B80-09 Idempotency commit branch+bytes hash, replay tanpa duplikasi (CORE-017)
- [x] B80-10 Akses product.manage/tenant/cabang retail aktif + audit metadata impor (CORE-012/042)

Migrasi 0055, build/vet/suite lama -race lulus. Tidak membuat fungsi tes baru; impor fitur baru belum diuji khusus. Satu baris CSV = satu produk/varian baru; kategori/multi-varian/unit tambahan belum. Frontend tidak diubah.

### Batch backend 10 task — pengelolaan katalog (implementasi)
- [x] B70-01 Detail kategori/status/tax/versi tenant (CORE-007)
- [x] B70-02 Edit nama kategori dengan If-Match/audit (CORE-007)
- [x] B70-03 Restore kategori arsip, konflik nama aktif dijaga (CORE-007/043)
- [x] B70-04 Detail produk termasuk category/tax/track_stock/status/versi (CORE-008)
- [x] B70-05 Edit nama/kategori produk dengan validasi kategori aktif tenant (CORE-008)
- [x] B70-06 Status produk active/inactive/archived; blok nonaktif bila dipakai order open (CORE-008/043)
- [x] B70-07 Edit nama/SKU varian dengan versi/audit/unique case-insensitive (CORE-008)
- [x] B70-08 Edit allow_fractional dengan versi dan blok pecahan apotek (RTL-003/APT-002)
- [x] B70-09 Daftar multi-barcode varian/unit dengan cursor barcode (CORE-008/RTL-002)
- [x] B70-10 Hapus barcode memakai versi varian dan audit, snapshot histori utuh (CORE-008)

Build/vet/suite lama -race lulus. Tidak membuat fungsi tes baru; endpoint batch belum diuji khusus. Frontend tidak diubah, tidak ada migrasi baru.

### Batch backend 10 task — diskon order/checkout (implementasi)
- [x] B60-01 Penyimpanan diskon nominal/persen order tanpa PIN persisten (CORE-015/RST-003)
- [x] B60-02 Penyimpanan diskon per baris dengan versi order (CORE-015)
- [x] B60-03 Hapus diskon lewat kind none/value 0 tanpa endpoint destruktif (CORE-015)
- [x] B60-04 Alasan diskon maksimal 200 karakter dan audit perubahan draft (CORE-015/042)
- [x] B60-05 Pembacaan diskon header/baris konsisten dengan versi order (RST-003)
- [x] B60-06 Pratinjau diskon baris/alokasi transaksi sebelum tax/SC (CORE-015/RST-009)
- [x] B60-07 Preview menolak nominal discount melebihi nilai item/subtotal (CORE-015)
- [x] B60-08 Checkout menerima approval PIN dan menerapkan batas peran/policy saat bayar (CORE-015)
- [x] B60-09 Snapshot diskon/alokasi/alasan/penyetuju pada sale/report melalui shared Finalize (CORE-015/038)
- [x] B60-10 Hash idempotency checkout mencakup approval/payment/versi; data PIN tidak disimpan pada order/audit (CORE-017)

Migrasi 0054, build/vet/suite lama backend -race lulus. Tidak membuat fungsi tes baru; alur baru checkout/diskon campuran belum diuji khusus. Frontend tidak diubah.

### Batch backend 10 task — checkout restoran (implementasi awal, belum uji alur khusus)
- [x] B50-01 Endpoint checkout restaurant dengan izin sale.create/modul/cabang (RST-010)
- [x] B50-02 Validasi versi/order OPEN, item aktif dan blok NOT_SENT/CANCELLED (RST-003/010)
- [x] B50-03 Pembayaran masuk shift terbuka terminal pada cabang pembayaran (RST-003)
- [x] B50-04 Harga/nama/satuan/modifier/note snapshot order diteruskan sale_lines (RST-004, §13.6)
- [x] B50-05 SC per jenis/cabang diterapkan pricing dan snapshot konfigurasi pada sales (RST-009)
- [x] B50-06 Reservasi active dikonsumsi atomik sebelum pengurangan SALE, rollback kegagalan mempertahankan reservasi (RST-013)
- [x] B50-07 Pembayaran tunai/noncash/split/change/rounding melalui shared Finalize (CORE-018/019/021)
- [x] B50-08 Order completed/paid/sale_id/closed_at atomik, meja derived NEEDS_CLEANING (RST-010)
- [x] B50-09 Idempotency key checkout + versi/payload hash, hasil/audit commit bersama (RST-010/CORE-017)
- [x] B50-10 Detail sale/public receipt menampilkan modifiers snapshot dan bill payment_blocked sesuai NOT_SENT/ongkir (CORE-022/RST-010)

Migrasi 0053 diterapkan; build/vet/suite backend lama -race lulus. Tidak membuat fungsi testing baru; checkout restoran belum diuji end-to-end/konkurensi khusus, jangan dianggap siap produksi. Ongkir nonzero diblokir, diskon order/split bill/recovery/FEFO masih terbuka. Frontend tidak diubah.

### Batch backend 10 task — kontrak pajak/struk/koreksi (selesai implementasi)
- [x] B40-01 Baca penugasan pajak produk dan versi master (CORE-016)
- [x] B40-02 Baca penugasan pajak kategori dan versi master (CORE-016)
- [x] B40-03 Reaktivasi aturan pajak arsip dengan If-Match/audit (CORE-016/043)
- [x] B40-04 Detail sale_line menampilkan snapshot nama/tarif/mode pajak (CORE-016/022)
- [x] B40-05 Detail sale_line menampilkan gross/diskon baris/alokasi/neto/tax/SC snapshot (CORE-022)
- [x] B40-06 Detail sale_line menampilkan nominal sudah direfund dari histori (CORE-028)
- [x] B40-07 Detail pembayaran menampilkan verification manual/sistem (CORE-019/037)
- [x] B40-08 Struk publik menampilkan rincian diskon/pajak/SC per baris tanpa HPP/PII tambahan (CORE-022/023)
- [x] B40-09 Riwayat retur per penjualan dengan cursor/pelaku/penyetuju/alasan/nominal (CORE-028)
- [x] B40-10 Detail void penjualan dengan cabang/pelaku/approval/alasan/waktu (CORE-027)

Tanpa fungsi testing baru; build/vet/suite lama backend -race lulus, endpoint baru belum diuji khusus. Frontend tidak diubah. Task gabungan yang memiliki sisa tetap terbuka.

### Batch backend 10 task — cakupan pajak (selesai implementasi)
- [x] B30-01 Buat aturan pajak khusus nama/tarif/mode Owner (CORE-016)
- [x] B30-02 Daftar aturan khusus termasuk arsip dengan cursor (CORE-016)
- [x] B30-03 Detail aturan khusus tenant/versi (CORE-016)
- [x] B30-04 Edit aturan khusus If-Match/audit, histori transaksi tetap snapshot (CORE-016)
- [x] B30-05 Arsip aturan setelah seluruh penugasan dilepas (CORE-016/043)
- [x] B30-06 Pasang aturan khusus produk, validasi FK tenant dan versi produk (CORE-016)
- [x] B30-07 Lepas aturan produk via tax_rule_id null, kembali fallback kategori/bisnis (CORE-016)
- [x] B30-08 Pasang/lepas aturan kategori dengan versi/audit/FK tenant (CORE-016)
- [x] B30-09 Finalisasi prioritas produk→kategori→default bisnis, snapshot pajak per baris dan laporan inklusif per baris (CORE-016)
- [x] B30-10 Pratinjau tagihan restoran memakai resolver pajak cakupan yang sama (RST-009/CORE-016)

Migrasi 0052 diterapkan, build/vet/suite backend lama -race lulus. Tidak ada fungsi tes baru; skenario campuran scoped tax/konkurensi/performa belum diuji khusus. Frontend tidak diubah; cakupan per cabang/checkout restoran tetap terbuka.

### Batch backend 10 task — detail/riwayat/keamanan sesi (selesai implementasi)
- [x] B20-01 Baca aturan pajak aktif bisnis, null bila belum dikonfigurasi (CORE-016)
- [x] B20-02 Daftar kategori arsip dengan cursor khusus pengelola (CORE-007/043)
- [x] B20-03 Detail varian tenant/status/atribut/versi tanpa HPP (CORE-008)
- [x] B20-04 Detail satuan harga/faktor/arsip/versi/penanda used (CORE-009/011)
- [x] B20-05 Daftar cabang dengan versi, termasuk status nonaktif dan cakupan Manager (CORE-003)
- [x] B20-06 Daftar sesi sendiri tanpa token/hash, cursor dan current flag (CORE-004)
- [x] B20-07 Cabut sesi lain akun sendiri, pertahankan sesi sekarang, audit atomik (CORE-004)
- [x] B20-08 Riwayat ekspor peminta sendiri/cabang tenant dengan cursor; unduhan tetap revalidasi izin (CORE-044)
- [x] B20-09 Riwayat sesi opname per cabang/status/versi dengan cursor (CORE-031)
- [x] B20-10 Detail pengeluaran dengan akses tenant/cabang dan informasi approval (CORE-041)

Batch ini tidak menambah fungsi tes baru. Build/vet/suite lama backend -race lulus; pengujian spesifik endpoint belum. Tidak ada perubahan frontend. Checkout restoran/apotek/platform dan tugas pokok lain tetap terbuka.

### Batch backend 10 task — pengelolaan data (selesai implementasi)
- [x] B10-01 Detail pelanggan tenant, termasuk status anonim (CORE-035)
- [x] B10-02 Edit pelanggan dengan If-Match, blok edit anonim dan audit tanpa PII (CORE-035)
- [x] B10-03 Pencarian pelanggan nama/HP literal + cursor (CORE-035)
- [x] B10-04 Detail supplier tenant/versi/status arsip (CORE-033)
- [x] B10-05 Pencarian supplier nama/kontak literal + cursor (CORE-033)
- [x] B10-06 Daftar undangan tanpa token/hash; Manager hanya undangan sendiri (CORE-004)
- [x] B10-07 Daftar terminal cabang aktif/nonaktif/arsip/all + cursor/versi (CORE-003)
- [x] B10-08 Baca profil nama bisnis dan versi (§18.1)
- [x] B10-09 Edit nama bisnis Owner dengan versi/audit, snapshot struk tetap (CORE-002)
- [x] B10-10 Arsip kategori lewat API dengan If-Match/audit, histori produk tetap (CORE-007/043)

Batch ini tidak menambah fungsi tes baru. Build/vet/suite backend lama -race lulus; pengujian khusus endpoint baru masih terbuka. Frontend tidak diubah. Checklist fase campuran tetap tidak dianggap selesai bila prasyarat lainnya belum.

- **Frontend:** aktif sesuai arahan pengguna 13 Oktober 2026; fondasi §1.8 dan redesign layout admin/login Soft UI biru didahulukan. Status halaman dan batas implementasi dicatat di §1.8.

- **Prioritas pengerjaan:** seluruh fase backend mengikuti roadmap terlebih dahulu; frontend ditangani agent lain dan tidak boleh disentuh pada pengerjaan backend. Fase 5 tetap backlog, dan validasi bisnis/regulasi/perangkat tidak dianggap selesai hanya karena backend tersedia.

- **Fase 0:** keputusan default dicatat di `docs/DECISIONS.md`. Wawancara pelanggan, perangkat acuan, dan validator eksternal belum.
- **Fase 1:** fondasi backend (modular monolith Go, logger, errors, authz, idkit, money, httpx, pricing engine) → jalan. Mesin harga backend + frontend berbagi vektor uji dan menghasilkan Rp80.900 persis sesuai PRD §13.8. Migrasi SQL ada di `backend/internal/platform/migrate/sql` (di-embed; `make migrate-up/down/status`). Login/logout/me dengan sesi server-side, CSRF, dan rate limit sudah jalan dan diuji manual terhadap PostgreSQL lokal. Uji integrasi PostgreSQL otomatis, katalog, dan PIN atasan sudah tersedia. Belum: registrasi/OTP, pengiriman undangan via penyedia email/HP, penyempurnaan default/edit satuan tambahan.
- **Fase 2:** backend stok, finalisasi idempoten, pembayaran, kas masuk/keluar dan tutup shift tersedia. UI kasir, operasi shift, diskon dengan PIN, penyesuaian stok, kartu stok, stok minimum, dan opname tersedia. Backend dan UI konfigurasi ambang diskon/kas keluar/selisih shift serta pembulatan tersedia; cakupan pajak kategori/produk belum. **Fase 3:** backend master area/meja mulai dikerjakan; alur pesanan/KDS belum. **Fase 4:** belum dimulai.

---

## Fase 0 — Validasi & Keputusan

- [ ] Wawancara 5–8 usaha per segmen (retail, kafe, apotek) untuk memvalidasi persona (§3)
- [x] ~~Catat keputusan default TBK yang memblokir desain~~ → `docs/DECISIONS.md`
  - [ ] TBK-04 izin stok negatif retail (default: dilarang)
  - [ ] TBK-05 pembulatan tunai (default: aktif, Rp100)
  - [ ] TBK-06 penomoran struk tanpa celah (default: ya)
  - [ ] TBK-08 lokasi data / penyedia infrastruktur
  - [ ] TBK-01 paket & harga (untuk landing page)
  - [ ] TBK-03 bridge printer lokal (default: tidak di MVP)
  - [ ] TBK-10 penyedia email & kanal notifikasi lead
- [x] ~~Tetapkan nilai default (§23 no. 10–12)~~ → `docs/DECISIONS.md`
- [ ] Siapkan perangkat acuan: 2 printer thermal 58/80 mm, 2 scanner HID, 1 tablet Android, 1 iPad, 1 PC Windows (§21.2)
- [ ] Uji perangkat awal: cetak via browser dan scanner HID (ASM-05, ASM-06)
- [ ] Mulai validasi eksternal: konsultan pajak, apoteker + ahli regulasi, penasihat hukum UU PDP (§21.3)

---

## Fase 1 — Fondasi

### 1.1 Setup proyek & infrastruktur
- [x] Repo monorepo: `backend/` (Go), `app/` (React SPA), `landing/` (statis/prerender)
- [x] ADR: pilihan arsitektur modular monolith (`docs/adr/0001`)
- [x] ADR: pilihan JavaScript, Vite, React Router dan React Context/state (§16.9) → `docs/adr/0004-frontend-soft-ui.md`
- [x] ADR: tool migrasi DB Go (§16.6). `docs/adr/0004-go-migration-runner.md` menetapkan runner Go yang sudah ada, SQL embedded, up/down/status via Makefile; bukan psql.
- [x] Binary Go dengan mode `api` dan `worker` (§16.1)
- [x] Struktur modul internal: identity, org, catalog, pricing, sales, payments, inventory, purchasing, shift, customers, restaurant, pharmacy, reporting, audit, leads (§16.2) — direktori sudah ada
- [x] Docker Compose lokal (Go + PostgreSQL) → `docker-compose.yml`
- [x] CI: lint (`go vet`), unit test (`go test -race`), uji vektor pricing Go + JS → `.github/workflows/ci.yml`
- [x] Logging terstruktur `log/slog` dengan request_id; larangan PII di log (NFR-018, NFR-019)
- [x] Metrik latensi/error per endpoint (NFR-018). Middleware in-process per method+route pattern, counter request/5xx dan histogram latency Prometheus text di `GET /internal/metrics` (loopback-only), tanpa provider/dependency eksternal; ada tes race/cardinality
- [x] Graceful shutdown (NFR-022, E25)
- [x] DILEWATI / TERBLOKIR — Backup: WAL archiving + backup harian, retensi 30 hari (§16.7, NFR-008) — infra deploy **Penutupan administratif, BUKAN selesai/tervalidasi. Memerlukan infrastruktur penyimpanan/WAL/retensi dan restore; belum dideploy/divalidasi.**

### 1.2 Konvensi data & API
- [x] Konvensi tabel: UUIDv7, tenant_id NOT NULL, created_at/updated_at/created_by, version (§15, §13.13) — migrasi baseline + ADR 0002
- [x] FK komposit menyertakan tenant_id (§13.13) — pola sudah dipakai di `0001_baseline.up.sql`
- [x] Uang BIGINT sen, kuantitas NUMERIC(14,3) / int64 milli-unit di Go (§13.7) — paket `platform/money` + ADR 0002
- [x] Format error seragam `{code, message, details[], request_id}` + daftar kode error stabil (§16.4) — paket `platform/errors` + `httpx.WriteError`
- [x] Paginasi cursor terpasang pada daftar operasional; timestamp+ID dipakai untuk urutan kejadian (§16.4). Beberapa detail agregat tetap bounded, bukan paginated
- [x] IMPLEMENTASI MVP — Spesifikasi OpenAPI /api/v1 (§16.4): seluruh 209 operasi terdaftar memiliki schema/status/media, termasuk raw CSV, CSV download dan SSE; generator tetap terhubung rute. 99 operasi pernah verified, 110 implemented-not-tested. Validasi menyeluruh/testing DITUNDA, bukan kontrak production-certified.
- [x] Optimistic locking If-Match/version dan 409 tersedia pada perubahan master/draft/order (CORE-008). Harga unit masih opsional untuk klien lama
- [x] Idempotency-Key terpasang pada aksi finansial/stok terkait, konflik isi berbeda/replay teruji; worker membersihkan expired hanya setelah minimal 72 jam, batch 500/SKIP LOCKED (§13.12, CORE-017). Posting penerimaan mendukung replay eksplisit dengan Idempotency-Key + If-Match; key sementara opsional untuk klien lama. Posting opname kini mendukung replay eksplisit key/versi juga; key sementara opsional, tes replay khusus belum ditambah
- [x] Tabel `outbox_jobs` siap untuk pola outbox (§13.12, §16.6)
- [x] Pool.InTx retry deadlock/serialization failure maksimal 3 retry dengan backoff (§13.5, E24)
- [x] Tabel `document_counters` dengan row lock (CORE-046)

### 1.3 Identity & autentikasi
- [x] Skema `users`, `sessions`, `invitations` (CORE-001, CORE-004) — migrasi baseline
- [x] ADR sesi server-side (`docs/adr/0003`)
- [x] DILEWATI / TERBLOKIR — Handler registrasi, verifikasi email/OTP (CORE-001) **(blok: kanal OTP)** **Penutupan administratif, BUKAN selesai/tervalidasi. Provider pengiriman dan verifikasi kepemilikan email/OTP dilewati; registrasi/komunikasi eksternal belum lengkap.**
- [x] Hash kata sandi argon2id, min 8 karakter, cek daftar kata sandi umum (NFR-011). Daftar umum hanya ~30 entri tertanam (shortcut); endpoint `PUT /auth/password` mencabut sesi lain
- [x] Login limiter 10/menit/IP+akun terpasang (NFR-011). In-memory per instance, distributed limiter produksi belum
- [x] Middleware sesi/CSRF double-submit terpasang pada endpoint terautentikasi (§16.5). Login Origin check/security hardening tetap terbuka
- [x] Pencabutan sesi saat pengguna dinonaktifkan ≤ 60 detik (CORE-004). Status akun dicek pada setiap request, jadi berlaku pada request berikutnya; ada tes
- [x] Skema PIN otorisasi atasan di `memberships` (CORE-006)
- [x] Handler verifikasi PIN + locking 15 menit setelah 5× gagal (CORE-006); uji HTTP persetujuan dan lockout tersedia

### 1.4 Organisasi & akses
- [x] Skema tenant (`businesses`) + `enabled_modules` + `first_sale_completed_at` (CORE-002)
- [x] Backend GET/PUT jenis usaha Pemilik dengan versi/audit/modul turunan; finalisasi mengisi first_sale_completed_at atomik (rollback kegagalan), backfill histori + trigger pengunci jenis/modul teruji (§5.3, CORE-002). Onboarding bisnis baru belum
- [x] Helper middleware modul aktif tersedia dan terpasang; `module_guard_test.go` menguji 63 rute restoran/apotek: bisnis retail selalu 403 MODULE_DISABLED (CORE-002)
- [x] Skema cabang (zona waktu, header/footer struk) dan terminal (CORE-003)
- [x] Skema keanggotaan, undangan, `membership_branches` (CORE-004)
- [x] DILEWATI / TERBLOKIR — Backend undangan email 7 hari, token hash sekali pakai/revoke/accept, pembatasan Manajer/cabang dan audit tersedia; kontak akun penerima dicek, bisnis aktif divalidasi. Pengiriman email/HP dan verifikasi akun/OTP belum (CORE-004) **Penutupan administratif, BUKAN selesai/tervalidasi. Provider pengiriman dan verifikasi kepemilikan email/OTP dilewati; registrasi/komunikasi eksternal belum lengkap.**
- [x] Satu definisi izin di kode sesuai matriks §7.2 → `platform/authz`
- [x] Kerangka middleware tenant context → `platform/tenant`
- [x] Rangkai middleware urutan: auth → tenant & keanggotaan → modul → izin + cabang (§16.5). `middleware_chain_test.go` menguji short-circuit CSRF/auth/tenant/modul/izin/cabang dan jalur sukses.
- [x] Log penolakan izin/tenant/modul/cabang dipusatkan di `httpx.WriteError` untuk respons 403 (CORE-005). Hanya method, route pattern, dan kode error; tes memastikan path mentah, query token, body, dan pesan sensitif tidak dicatat.
- [x] Backend GET/PUT keanggotaan dengan versi, audit peran/penugasan/status, pembatasan Manajer/cabang, penjagaan Pemilik terakhir dan disable efektif pada request berikutnya (CORE-004/005). Undangan belum
- [x] DILEWATI / TERBLOKIR — Row-Level Security PostgreSQL sebagai lapisan kedua + uji performa (§13.13) **Penutupan administratif, BUKAN selesai/tervalidasi. Menunggu pemisahan role DB non-owner/migrator dan propagasi konteks tenant pada semua jalur query; belum diimplementasikan, bukan aman oleh RLS.**
- [x] Uji otomatis: matriks peran × endpoint dibangkitkan dari definisi izin (CORE-005). 108 endpoint ber-guard × 6 peran dibaca dari kode; endpoint khusus modul restoran/apotek belum dicakup
- [x] Uji otomatis lintas tenant untuk setiap endpoint (NFR-013). Header bisnis lain ditolak 403 di 188 rute; ID sumber daya bisnis lain dibaca = 404 untuk 5 jenis sumber daya, bukan seluruh endpoint detail

### 1.5 Audit
- [x] Login berhasil/gagal/rate-limited dan logout diaudit tanpa email/password/token; sesi login/logout dan audit atomik. IP peer tervalidasi/user-agent terbatas diteruskan ke semua audit dari request terautentikasi (CORE-042)
- [x] Tabel `audit_logs` append-only dengan trigger yang menolak UPDATE/DELETE (CORE-042)
- [x] Field: pelaku, pemberi otorisasi, tenant, cabang, terminal, waktu UTC, entitas, diff, alasan, IP/UA (CORE-042)
- [x] DILEWATI / TERBLOKIR — REVOKE UPDATE/DELETE pada role aplikasi setelah role dibuat (CORE-042, §16.3) **Penutupan administratif, BUKAN selesai/tervalidasi. Role runtime deployment belum ditentukan; trigger append-only bukan pengganti least-privilege.**
- [x] Uji integrasi: DELETE pada audit ditolak oleh DB. Juga UPDATE dan TRUNCATE (audit, stock_movements, pharmacy_batch_movements); TRUNCATE ditambal di migrasi 0061

### 1.6 Katalog
- [x] Kategori satu level, arsip bila dipakai (CORE-007) → `internal/catalog`
- [x] Produk → varian, SKU unik case-insensitive, status Aktif/Nonaktif/Arsip, `track_stock` (CORE-008) — migrasi 0003
- [x] Multi-barcode per varian, unik per tenant, pesan konflik menyebut varian lain (CORE-008)
- [x] Satuan dasar/tambahan, konversi milli, harga per satuan dan faktor used terkunci DB/API tersedia (CORE-009)
- [x] Harga jual satuan dasar + optimistic locking + riwayat perubahan ke audit (CORE-011); harga satuan tambahan belum
- [x] Kebijakan hapus vs arsip untuk semua master data (CORE-043). `docs/MASTER-DATA-LIFECYCLE.md` + tes inventaris seluruh rute DELETE; master historis utama dilarang punya DELETE. Supplier terpakai penerimaan biasa/batch ditolak 409; produk/varian/satuan/cabang/terminal/anggota tanpa DELETE; kategori/pajak/area/meja memakai arsip

### 1.7 Mesin harga (pricing)
- [x] Implementasi urutan perhitungan §13.8 di Go (int64 sen, half-up) → `internal/pricing/engine.go`
- [x] Alokasi diskon transaksi proporsional + selisih pembulatan ke baris konsisten (§13.8 langkah 3)
- [x] Pajak inklusif/eksklusif per baris, service charge opsional termasuk dasar pajak (CORE-016, RST-009)
- [x] Mode pembulatan dapat dikonfigurasi tanpa ubah skema (`money.RoundingMode`)
- [x] Aturan default bisnis dan aturan khusus produk/kategori nama/tarif/mode dengan snapshot per baris tersedia (CORE-016). Cakupan cabang dan uji mixed-tax belum
- [x] Berkas vektor uji bersama `backend/internal/pricing/testdata/vectors.json` + contoh §13.8 = Rp80.900 
- [x] Port mesin harga ke frontend (`app/src/pricing/engine.js`) + vektor uji identik
- [x] Cakupan uji unit mesin harga ≥ 90% (NFR-020) — unit test + 3 vektor bersama

### 1.8 Frontend dasar
- [x] Scaffold `app/` dengan `package.json` + skrip uji vektor Node (sebelum tool build final dipilih)
- [x] Utilitas format uang/kuantitas berbasis BigInt (§16.9, NFR-017) → `app/src/money.js`
- [x] Shell aplikasi React: login, pilih bisnis/cabang/terminal (§18.1); layout Soft UI biru, sidebar/header, menu mobile dengan dialog native. `GET /workspace` membatasi cabang/terminal melalui sesi, tenant dan penugasan
- [x] Klien API: error seragam, Idempotency-Key per percobaan, version header; unduhan CSV terautentikasi dan pencabutan UI saat 401 diuji
- [x] Kamus istilah per modul ("Produk"/"Menu"/"Obat") (§5.1), implementasi dan tes tersedia
- [x] Code splitting per modul usaha (§16.9): seluruh halaman admin memakai React.lazy, termasuk restoran (`/meja`, `/meja/kelola`, `/dapur`, `/pesanan`, `/menu-habis`) dan apotek (`/batch`)
- [x] Indikator koneksi melalui event online/offline (NFR-021); ditampilkan pada header. Error server memiliki pesan request dan tombol coba lagi; deteksi perangkat bukan pemeriksaan kesehatan server
- [x] Komponen dasar: fokus terlihat, label, target sentuh ≥ 44 px (NFR-015, §17.1). Verifikasi keyboard/mobile di browser nyata masih perlu dilakukan
- [x] Pola keadaan kosong, loading, error/403 dan retry bersama (§18.3), termasuk kegagalan pemeriksaan sesi

#### Status halaman admin (13 Oktober 2026)

- [x] Redesign login dan layout admin Soft UI biru; Anti Slop dinonaktifkan sesuai arahan pengguna
- [x] Dashboard dari laporan penjualan/pembayaran/performa produk, memakai zona waktu cabang; data kosong dan histori tidak lengkap dipertahankan
- [x] Katalog: cari nama/SKU/barcode, tambah produk satu varian, ubah harga dengan If-Match. UI satuan tambahan: buat faktor konversi, harga jual dan barcode satuan tersedia. Generator hingga 3 atribut/100 kombinasi dengan SKU/harga editable tersedia. Cetak label barcode tersedia; impor belum memiliki UI
- [x] Supplier: daftar dengan cursor, tambah/edit dan arsip dengan versi
- [x] Pelanggan: daftar dengan cursor, tambah dan anonimisasi Pemilik dengan konfirmasi. Pemilih pelanggan pada checkout tersedia. Edit/pencarian/histori per pelanggan belum
- [x] Persediaan: saldo/nilai terkini, filter minimum/negatif, periode pergerakan, ekspor CSV, penerimaan barang dasar, penyesuaian idempoten, kartu stok, stok minimum/pengadaan dan opname
- [x] Laporan: penjualan, pembayaran, koreksi/diskon, performa produk dan stok; periode/cabang, izin laba/biaya, ekspor CSV. Agregasi kategori, antrean ekspor latar, periksa status dan unduhan hasil tersedia; riwayat antrean lintas sesi belum
- [x] Pengeluaran: daftar periode/cabang dengan cursor, tambah dari kas shift atau di luar kas, approval PIN, izin Kasir, idempotency dan pemulihan request dengan kunci sama di sessionStorage per pengguna/bisnis
- [x] Pengaturan: profil bisnis/akun/cabang/terminal, perubahan aturan pajak bisnis, PIN atasan, kebijakan operasional/pembulatan, tambah/edit cabang, header/footer struk, tambah/nonaktifkan terminal. Daftar terminal nonaktif, metode bayar dan konsol platform belum; akses keanggotaan dan undangan tersedia di halaman Anggota
- [x] Anggota: daftar dengan cursor, ubah peran/status/penugasan cabang memakai If-Match; Manajer terbatas pada peran/cabang yang diizinkan. Buat/cabut undangan manual dan terima token pada Pengaturan tersedia; riwayat undangan belum
- [x] Riwayat transaksi: pencarian struk/periode lokal/cabang dengan cursor, detail snapshot barang/satuan/pembayaran, void dengan PIN, retur per barang/kondisi dan retry koreksi idempoten. QR dan retur split/apotek belum
- [x] Log aktivitas: filter UTC/cabang/tindakan, cursor dan detail sebelum/sesudah, memakai endpoint backend
- [x] Kasir dasar: pencarian/scan barcode satuan, pemilih varian/satuan, deteksi HID, keypad kuantitas/pembayaran, opsi scan ganda/bunyi, draft server-side, tahan/lanjutkan, ubah kuantitas, catatan/diskon, undo hapus, buka/tutup shift, kas masuk/keluar, pembayaran tunai/nontunai/split, approval diskon, dan retry idempoten. Struk digital/QR tersedia; cache offline keranjang/katalog tersedia; kalibrasi printer dan startup aplikasi tanpa koneksi tetap backlog
- [x] Tes unit klien API/uang/rekap dan render React untuk login/menu peran/keadaan kosong/error; build Vite
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: Uji browser desktop/tablet/mobile dan login dengan PostgreSQL nyata; lingkungan agen menolak socket Vite/PostgreSQL dan peluncuran Chromium

Halaman di atas tidak menyelesaikan seluruh Fase 2. Task kalibrasi cetak, onboarding/impor, penerimaan barang, landing page, restoran dan apotek tetap mengikuti bagian roadmap berikut.

---

## Fase 2 — MVP Retail

### 2.1 Onboarding
- [x] Wizard konfigurasi bisnis yang sudah ada: nama bisnis → jenis usaha → cabang/terminal → pajak/pembulatan/struk + service charge restoran → produk awal (§8.1), di `/mulai`. Memakai API berversi yang ada, langkah bisa dilewati; registrasi/pembuatan bisnis baru tetap terpisah dan OTP dilewati. Uji browser nyata belum.
- [x] Data demo yang dapat dihapus (§8.1). `POST/DELETE /onboarding/demo`: 3 produk contoh per jenis usaha, idempoten, dihapus dengan diarsipkan, bentrok SKU pengguna ditolak 409; belum diuji di browser
- [x] Checklist onboarding yang dapat dilewati (CORE-001). 5 langkah dihitung server dari data nyata, panel di dashboard pemilik dengan tombol Sembunyikan; belum diuji di browser
- [x] Event onboarding untuk metrik M2: `POST /onboarding/events` menyimpan milestone completed/skipped ke audit append-only, whitelist 5 langkah, deduplikasi per bisnis/langkah/status; izin Pemilik dan isolasi tenant diuji. M2 waktu transaksi pertama tetap dari timestamp server, bukan klaim event klien.

### 2.2 Impor produk
- [x] Backend template CSV/preview baris/commit atomik/batas 5.000 dan 5 MiB tersedia (CORE-012), retail satu produk-varian per baris. UI/category/multi-variant import belum
- [x] Backend CSV stok awal OPENING_BALANCE dengan HPP wajib stok positif, audit/key atomic (CORE-012). Verifikasi khusus impor belum

### 2.3 Layar kasir
- [x] Endpoint pencarian (nama, SKU, barcode, sebagian) p95 ≤ 300 ms pada 50.000 varian (CORE-013, NFR-002): benchmark HTTP lokal 2 tenant ×50k, 20 sampel/query setelah warmup, p95 26–189 ms dengan -race. Barcode candidate lookup sekali via indeks tenant+barcode; produksi/beban konkuren belum tervalidasi.
- [x] Kolom scan dengan fokus default; deteksi scanner HID melalui input cepat berurutan + Enter, tanpa menghalangi input manual (CORE-013, RTL-004). Kalibrasi scanner perangkat nyata belum
- [x] Pencegahan scan ganda < 300 ms, dapat dimatikan melalui opsi kasir; batas waktu dan scan berbeda diuji (E5)
- [x] Feedback bunyi/visual sukses/gagal; bunyi dapat diaktifkan/dimatikan melalui opsi kasir (RTL-004). Verifikasi audio/scanner pada perangkat nyata belum
- [x] Keranjang draft tersimpan di server per terminal, tahan refresh; versi If-Match dan penyelesaian atomik bersama finalisasi tersedia (CORE-014)
- [x] Scan ulang menambah qty; catatan baris/transaksi ≤ 200 karakter disimpan pada draft (CORE-014). Backend snapshot catatan penjualan final tersedia, validasi 200 karakter dan kesamaan draft/finalisasi diuji; agent frontend perlu meneruskan field note
- [x] Endpoint hold/lanjutkan draft, maksimal 10 tertahan dan satu aktif per terminal; mutasi paralel dikunci per terminal (CORE-014)
- [x] Backend dan UI diskon baris/transaksi nominal/persen + persetujuan PIN tersedia (CORE-015). Batas diskon dapat diatur Pemilik melalui UI dengan versi If-Match. Pratinjau kasir/pembayaran/cache memakai mesin harga bersama sehingga diskon tersimpan langsung mengurangi subtotal; keranjang selalu menampilkan subtotal bruto, diskon gabungan (termasuk Rp0), dan total akhir berlabel estimasi sebelum pajak/biaya/pembulatan. Regresi Rp8.000 − Rp1.000 = Rp7.000, diskon berurutan/satuan tambahan, batas nol dan harga belum tersedia diuji
- [x] Layout kiri pencarian/grid, kanan keranjang/total, total sebagai angka terbesar (§18.2)
- [x] Pintasan keyboard: Bayar, Tahan, Cari, Diskon, Hapus baris terakhir (§17.1)
- [x] Undo hapus baris 5 detik (§18.2)
- [x] Keypad numerik layar untuk kuantitas dan pembayaran split: digit/desimal, hapus digit, kosongkan, penguncian saat request; konversi presisi diuji (§17.1)
- [x] Mode offline MVP retail: cache katalog/satuan dan keranjang per akun/bisnis/cabang/terminal, pencarian cache, edit qty/hapus/tambah, tombol Bayar diblokir; sinkronisasi draft eksplisit dengan pemeriksaan versi dan rekonsiliasi respons terputus (§17.5, NFR-021). Sesi/katalog perlu dimuat online sebelumnya; startup aplikasi tanpa koneksi belum tersedia
- [x] Peringatan jam perangkat selisih > 5 menit dari header Date API dengan estimasi titik tengah request; header invalid dan request >10 detik diabaikan, batas peringatan diuji (E22)

### 2.4 Finalisasi & pembayaran
- [x] Backend snapshot note transaksi/baris, UTF-8 maksimal 200 karakter, pemeriksaan draft note dan rollback invalid note teruji (CORE-014, §13.6). Frontend tidak diubah
- [x] Finalisasi atomik `POST /sales/finalize`: validasi → lock saldo (urut ID) → harga → nomor struk → simpan → pergerakan stok → audit (CORE-017, §13.5). Draft server-side tersedia melalui draft_id/draft_version; jalur keranjang sekali jalan masih kompatibel
- [x] Snapshot baris SKU/nama/satuan/harga/pajak/HPP, modifier/catatan dan header bisnis/cabang/zona/header/footer/nama kasir tersedia (§13.6). Detail/API publik memakai snapshot, ReceiptPaper menampilkan modifier/catatan; tes escaping teks dan perubahan master/header/HPP tersedia. Data historis yang tidak lengkap tidak direkonstruksi palsu.
- [x] Respons STOCK_INSUFFICIENT dengan detail item (§13.5). Stok negatif diizinkan: belum (default TBK-04 = dilarang)
- [x] UI memeriksa harga/ketersediaan produk dan satuan saat membuka checkout serta sebelum finalisasi; harga berubah memperbarui subtotal dan meminta tinjau ulang, produk/satuan nonaktif menghalangi bayar (E3, E4). Harga akhir tetap ditentukan server pada finalisasi
- [x] Pembayaran tunai + kembalian serta tombol nominal cepat tersedia (CORE-018)
- [x] Backend pembulatan tunai sebagai komponen terpisah, toggle/kelipatan configurable Pemilik (default Rp100), hanya semua tunai, kompatibilitas policy lama dan noncash diuji (CORE-018, TBK-05). UI Pemilik untuk toggle dan kelipatan pembulatan tersedia
- [x] Pembayaran nontunai manual (qris/card/transfer), verification=MANUAL, tidak boleh melebihi sisa (CORE-019). Metode kustom belum
- [x] Split tender, kembalian hanya dari tunai (CORE-021)
- [x] State machine status transaksi & pembayaran, tolak INVALID_STATE_TRANSITION (§13.1, §13.2). Trigger database (migrasi 0062) untuk sales, payments, status retur, draf, dan pesanan restoran; AWAITING_PAYMENT/PENDING/FAILED belum ada karena pembayaran terintegrasi masuk Fase 5
- [x] UI pemulihan timeout: status tidak pasti, kirim ulang dengan kunci sama dari sessionStorage, keranjang terkunci (§13.12, E1)
- [x] Uji integrasi: idempotency (replay, isi beda, paralel), rollback penuh, SHIFT_NOT_OPEN
- [x] Uji konkurensi: 5 kasir berebut 3 unit terakhir; 100 finalisasi paralel → 100 nomor unik tanpa celah (CORE-046, §16.10)
- [x] Target performa finalisasi p95 ≤ 500 ms (NFR-001), pengukuran HTTP PostgreSQL lokal 30 sampel setelah warmup (satu item terlacak stok): p95 5,55 ms dengan -race. Termasuk stok/payment/counter/audit/idempotency; bukan validasi load produksi/multi-item konkuren.

### 2.5 Struk
- [x] Backend snapshot header bisnis/cabang/alamat/zona/header/footer saat finalisasi; detail/struk publik mengembalikan snapshot serta total diskon/SC, histori lama ditandai snapshot hilang (CORE-022, §13.6). Nama kasir tampilan tersedia pada snapshot transaksi baru; profil kosong/histori lama tetap null
- [x] Backend rate limit struk publik 60/menit/IP peer di PostgreSQL lintas instance, Retry-After/reset window dan cleanup worker; uji akses multi-instance tersedia
- [x] Backend otorisasi cetak ulang tenant/cabang + label SALINAN dan audit request, tanpa mengubah transaksi/stok (CORE-022)
- [x] Backend create/revoke link struk dan public JSON receipt; token salah/revoked 404, tanpa HP/HPP/kredensial, no-store (CORE-023)
- [x] Template cetak CSS 58 mm dan 80 mm: barang/qty/satuan/harga snapshot, total/pajak/pembulatan/kembalian, pratinjau dan cetak browser (CORE-022). Kalibrasi printer fisik belum
- [x] Cetak ulang Riwayat berlabel "SALINAN": frontend menunggu POST /sales/{id}/reprints dan audit berhasil sebelum membuka dialog cetak; kegagalan audit menghalangi cetak (CORE-022). Kalibrasi printer fisik belum
- [x] Backend tautan struk token 256 bit (hash DB), API publik data minimal, pencabutan semua link per transaksi oleh Pemilik dan audit teruji (CORE-023). Halaman /receipt#token dan QR checkout/Riwayat tersedia

### 2.6 Shift & kas
- [x] Buka shift: satu per terminal, kas awal wajib, terminal nonaktif ditolak; UI memuat status shift aktif, menampilkan waktu/kas awal/pembuka, dan mencegah formulir buka shift ganda (CORE-024, CORE-003)
- [x] Backend dan UI kas masuk/keluar dengan alasan/PIN dan pengaturan ambang oleh Pemilik tersedia (CORE-025)
- [x] Backend dan UI tutup shift dengan hitung buta, tunai neto, kas masuk/keluar/refund, selisih, catatan + PIN dan pengaturan ambang oleh Pemilik tersedia (CORE-026)
- [x] Implementasi shift tertutup terkunci dan catatan koreksi append-only terpisah tersedia (CORE-026, B500); riwayat/summary + catatan di /shift. Testing DITUNDA, belum lulus; migrasi 0065 sudah diterapkan lokal (B510).
- [x] Endpoint tutup paksa oleh Pemilik/Manajer, kasir ditolak (E7)
- [x] UI ringkasan lengkap `GET /api/v1/shifts/{id}/summary`: respons close membawa shift_id, UI memuat pembayaran per metode/verifikasi, kas awal/masuk/keluar, refund tunai, void, diskon dan cetak (§8.5). Tes API membuktikan ID close membuka ringkasan yang benar; browser nyata belum.
- [x] UI mencetak hasil tutup shift yang diterima dari `POST /shifts/close`: kas seharusnya, kas terhitung, dan selisih

### 2.7 Koreksi transaksi
- [x] Backend pencarian transaksi nomor struk/periode/cabang dan detail snapshot dengan sisa qty retur, penyamaran pelanggan via referensi dan proteksi biaya/tenant/cabang (CORE-028/035)
- [x] Backend batas hari retur configurable Pemilik (0=tanpa batas), perbandingan tanggal lokal cabang dan rollback tanpa refund/stok bila kedaluwarsa; uji WITA/batas hari tersedia (CORE-028). UI batas hari retur Pemilik tersedia pada Pengaturan
- [x] UI pencarian/detail penjualan, void dengan alasan/PIN, retur qty/kondisi per baris, informasi refund manual dan pemulihan timeout dengan kunci sama (CORE-027/028)
- [x] Batal draft dengan pemeriksaan versi; draft dibatalkan/completed tidak dapat diaktifkan ulang (CORE-027)
- [x] Backend void dalam shift asal terbuka: PIN atasan + alasan, SALE_VOID dari pergerakan historis, pembayaran REVERSED, penanda refund nontunai manual, nomor struk tetap, replay idempoten (CORE-027)
- [x] IMPLEMENTASI MVP — Retur/refund CORE-028: qty/snapshot/proporsi/residual, split refund, pencarian/cursor/detail, riwayat retur/void pada UI, struk digital/QR, alasan/approval dan cash shift tersedia. Apotek kembali karantina. Refund nontunai manual; provider otomatis di luar MVP. Testing terakhir DITUNDA, belum lulus.
- [x] Backend RETURN_RESTOCK/RETURN_DAMAGED dan return_status partial/full; stok rusak tidak menambah saldo jual (CORE-028)
- [x] Refund tunai wajib shift terbuka dan dikurangkan dari kas shift refund; nontunai asli dicatat manual (CORE-028, E10, E11). Split tender dan apotek diblokir
- [x] Uji perhitungan refund termasuk sisa pembulatan terakhir, retur parsial/penuh, barang rusak, kas refund dan konkurensi jumlah terakhir

### 2.8 Stok & pembelian
- [x] Backend posting opname Idempotency-Key + If-Match atomik dengan penyesuaian/audit, replay hasil dan konflik key/versi; cabang dicek sebelum replay (CORE-031/017). Kompatibilitas tanpa key dipertahankan; belum uji khusus replay opname
- [x] Backend PUT /stock/variants/{id}/opening-cost khusus Pemilik: isi HPP saldo positif yang masih NULL, alasan/audit/key idempotency, lock saldo dan larangan menimpa biaya. Histori HPP penjualan tetap tidak direkonstruksi (§13.11). Tes spesifik fitur belum ditambah sesuai prioritas fitur
- [x] Backend posting penerimaan replay idempoten dengan key/versi: hasil sama tanpa menambah stok/HPP/audit lagi, konflik versi ditolak dan akses cabang diperiksa ulang sebelum replay (CORE-029/017). Key opsional demi kompatibilitas frontend lama
- [x] Backend daftar/search penerimaan: cabang/tanggal dokumen/status/faktur eksak/supplier, cursor stabil pada timestamp sama, no-store dan akses cabang teruji (CORE-029). Frontend belum diubah
- [x] Backend GET /goods-receipts/{id} dan PUT draft: detail snapshot/status untuk memeriksa timeout posting, edit header/supplier/baris atomik, If-Match, audit, posted immutable via API dan isolasi tenant/cabang teruji (CORE-029)
- [x] `inventory_balances` + `stock_movements` append-only (trigger) dengan balance_after (CORE-032) — migrasi 0004
- [x] Backend dan UI penerimaan DRAFT → POSTED, konversi snapshot dan HPP moving average tersedia (CORE-029, §13.11). `/penerimaan` mencari dokumen periode/status/faktur, memuat/edit draft berversi, mengecek status posted, dan posting dengan key/payload/versi tersimpan untuk retry. Form pembuatan lama juga memakai idempotency. Browser nyata belum diuji.
- [x] Backend supplier: nama/kontak/alamat/catatan, CRUD dengan versi, cursor, audit dan isolasi tenant; yang dipakai penerimaan hanya dapat diarsipkan (CORE-033)
- [x] Backend dan UI penyesuaian stok dengan daftar alasan, other wajib catatan, PIN untuk izin bersyarat, retry idempoten dan penolakan stok negatif (CORE-030)
- [x] Backend dan UI stok opname: snapshot awal, input dengan system_qty_at_count/timestamp, If-Match, posting OPNAME_ADJUSTMENT, item tak dihitung tetap; konkurensi posting diuji (CORE-031). Apotek per batch ditunda ke APT-013
- [x] Endpoint dan halaman kartu stok tenant/cabang/varian dengan cursor (`GET /api/v1/stock/card`) tersedia (CORE-032)
- [x] Worker rekonsiliasi harian UTC saldo vs pergerakan (RETURN_DAMAGED non-saldo) dan pembayaran confirmed vs total/non-void, snapshot SQL tunggal, claim harian anti-duplikat, temuan persisten + alert log terstruktur; endpoint tenant berizin audit (CORE-032, NFR-012). Kanal alert eksternal belum
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: Uji unit HPP dan aturan stok ≥ 90% (NFR-020)

### 2.9 Modul retail
- [x] Backend PUT purchase-units nama/faktor/arsip dengan If-Match, audit, faktor used terkunci dan larangan arsip unit default; harga jual mendukung versi opsional untuk kompatibilitas klien lama. Versi tampil pada daftar unit (CORE-009/011, RTL-002). Tes spesifik belum ditambah
- [x] Backend GET/PUT default unit beli/jual per varian: nullable=satuan dasar, FK memastikan unit milik varian/tenant, harga jual wajib, If-Match/audit. Tidak menambah fungsi testing baru; verifikasi khusus API masih terbuka (RTL-002)
- [x] Backend varian hingga 3 atribut/100 kombinasi, generator SKU sebagai pratinjau editable, penyimpanan atribut dan nonaktif/aktif per kombinasi dengan versi (RTL-001)
- [x] UI produk multivarian: input hingga 3 atribut, pratinjau maksimal 100 kombinasi dari backend, edit SKU/harga dan penyimpanan seluruh kombinasi lewat POST /products (RTL-001)
- [x] Data atribut/varian aktif tersedia pada pencarian; kasir memilih varian melalui tile dengan nama/SKU/atribut dan satuan (RTL-001)
- [x] Backend/UI satuan jual tambahan, harga/barcode/konversi/snapshot retur, pemilih kasir, default beli/jual berversi tersedia (RTL-002). UI default di katalog, penerimaan memilih default beli dan tambah kasir memilih default jual; scan barcode dan pilihan satuan eksplisit tetap menang. Browser nyata belum.
- [x] UI kasir satuan tambahan: pemilih satuan jual, barcode→satuan, baris berbeda per satuan, scan ulang menambah qty satuan, subtotal memakai harga satuan, muat ulang harga dari server (RTL-002)
- [x] UI katalog satuan tambahan: buat faktor konversi, lihat faktor terkunci, atur harga jual, tambah barcode satuan (RTL-002)
- [x] Backend allow_fractional per varian (default false), draft/finalisasi menolak pecahan tanpa izin; snapshot kebijakan untuk retur, uji 0,375 × Rp14.000 = Rp5.250 (RTL-003). UI input berat ditunda
- [x] Cetak label barcode Code 128 via browser: A4 3×8, A4 2×7 dan thermal 50×30 mm; pilih barcode/SKU dan jumlah label dari hasil katalog (RTL-005). Kalibrasi printer dan kertas fisik belum
- [x] Implementasi stok minimum (RTL-006) tersedia: backend/UI threshold/reorder dan banner polling 60 detik untuk retail/restoran/apotek. Apotek dihitung dari batch SELLABLE yang ED-nya setelah tanggal lokal cabang, bukan saldo inti; pagination dan supplier batch terakhir tersedia. Testing B510 DITUNDA, belum lulus; notifikasi eksternal dilewati.

### 2.10 Pelanggan & pengeluaran
- [x] Fitur lokal pelanggan (CORE-035): create/detail/edit/cari/anonimisasi tanpa PII audit, customer_id pada finalisasi, pilihan checkout dan nama pada riwayat. Histori per pelanggan memakai GET /sales?customer_id dengan tenant/cabang/periode/cursor; tautan dari daftar pelanggan tersedia. Validasi legal/PDP eksternal tetap dilewati, bukan dinyatakan lulus.
- [x] UI pelanggan opsional pada checkout dengan cursor, customer_id dikirim pada finalisasi dan nama pelanggan tampil pada riwayat (CORE-035)
- [x] Backend pengeluaran tanggal/kategori/nominal/metode/catatan, kas shift membuat kas keluar atomik, outside_cash tidak mengubah kas/HPP, izin/PIN/idempotency dan daftar per periode/cabang (CORE-041). UI sumber dana kas shift/di luar kas, terminal aktif dan approval PIN tersedia; lampiran fase lanjutan

### 2.11 Laporan
- [x] IMPLEMENTASI MVP — Laporan penjualan/pembayaran/performa/stok/koreksi memakai query reporting masing-masing yang sama untuk JSON/CSV, batas tanggal lokal cabang §13.10, koreksi pada waktu koreksinya dan snapshot historis. Tidak menyatukan jenis laporan berbeda ke satu SQL raksasa; bukan kebutuhan PRD. Testing menyeluruh DITUNDA.
- [x] Backend laporan penjualan per periode/cabang/kasir/jam lokal: bruto/diskon/retur/neto/pajak/SC/pembulatan/count/rata-rata, void dikecualikan, snapshot rincian retur dan flag histori tidak lengkap (CORE-036)
- [x] Backend laporan metode pembayaran: penerimaan/refund/neto per tanggal/jam lokal cabang, pelaku, metode dan verifikasi; filter periode/cabang/kasir (CORE-037)
- [x] Backend laporan retur/void/diskon per pelaku, penyetuju, alasan dan tanggal kejadian lokal; filter periode/cabang/pelaku, diskon baris/transaksi terpisah, CSV teraudit dan proteksi formula spreadsheet (CORE-038)
- [x] IMPLEMENTASI MVP — CORE-039: saldo/nilai stok saat ini, minimum, pergerakan periode dan batch apotek JSON/CSV/UI tersedia; HPP unknown bukan nol, izin biaya dan label estimasi. Saldo/nilai historis tidak diminta CORE-039 dan tidak diklaim tersedia. HPP batch estimasi penerimaan terpisah dari stok inti. Testing DITUNDA.
- [x] Backend performa per produk/snapshot kategori, qty dasar terjual/retur, neto, HPP jual/retur restock snapshot, laba estimasi nullable dan jumlah produk tanpa HPP; izin laba, label estimasi dan CSV teraudit (CORE-040). Agregasi kategori dengan group_by=category, HPP saldo pascaretur dan kesamaan CSV/JSON produk/kategori sudah diuji; UI kelompok produk/kategori dan CSV tersedia
- [x] IMPLEMENTASI MVP — CORE-044: JSON/CSV memakai hasil query sama, audit ekspor, antrean laporan utama >100k dengan worker SKIP LOCKED, PostgreSQL artifact <=50MiB/retensi 7 hari/status/download revalidasi dan UI riwayat ekspor. Serializer 100005 baris pernah diuji; E2E besar kini DITUNDA. Streaming object storage bukan syarat CORE-044 dan tidak diklaim tersedia; batch/kitchen CSV sinkron memiliki batas eksplisit.
- [x] UI ekspor laporan latar belakang: buat antrean, periksa status pending/processing/completed/failed, unduh hasil CSV terautentikasi (CORE-044)
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: Uji laporan pembayaran vs tunai neto shift, tanggal retur lokal, dan laporan penjualan satu hari vs 3 shift tertutup serta batas tengah malam tersedia (CORE-036). Skenario lintas-hari/tax/diskon/refund gabungan seluruh shift belum
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: Uji query laporan penjualan PostgreSQL lokal 5.000 transaksi/hari ×5 lulus ≤3 detik (12–845 ms dengan -race). Target seluruh laporan/API, dataset multitenant besar dan beban konkuren belum divalidasi (NFR-004)

### 2.12 Pengaturan & platform
- [x] Backend GET/PUT /me/profile untuk nama tampilan, versi If-Match/audit tanpa menyalin nama dan snapshot nama kasir tidak berubah saat profil diedit; UI Pengaturan menyunting nama tampilan dengan If-Match dan memperbarui nama di header (§18.1)
- [x] Backend GET/PUT kebijakan operasional Pemilik: batas diskon kasir, ambang kas keluar, ambang selisih tutup shift serta toggle/kelipatan pembulatan tunai; If-Match, audit dan enforcement teruji (CORE-015/018/025/026). UI Pemilik tersedia, termasuk pembulatan tunai; versi server dipertahankan untuk seluruh kebijakan
- [x] IMPLEMENTASI pengaturan organisasi (§18.1): backend/UI cabang, terminal, profil, pajak/keanggotaan tersedia; B580 menambah konfigurasi aktif/nama untuk 4 metode manual MVP dan daftar terminal semua status/pagination/reactivate. Kode metode custom baru/payment gateway tidak dibuat. Testing DITUNDA; migrasi 0066 diterapkan lokal pada B590.
- [x] UI Pemilik tambah/edit cabang, zona waktu IANA, alamat, header/footer struk, tambah terminal dan perubahan status dengan If-Match (§18.1)
- [x] Backend dan UI GET /api/v1/audit-logs: periode UTC/filter pelaku/tindakan/entitas/cabang, cursor stabil, izin audit.view dan isolasi tenant/cabang tersedia; Owner saja untuk event tanpa cabang. Halaman UI filter periode/cabang/tindakan, cursor dan detail tersedia (CORE-042)
- [x] UI anggota Pemilik/Manajer: daftar cursor, ubah peran/status/cabang dengan versi, pembatasan peran Manajer, buat/cabut undangan manual, terima token oleh akun penerima (CORE-004)
- [x] DILEWATI / TERBLOKIR — Konsol admin platform dengan MFA wajib (CORE-045) **Penutupan administratif, BUKAN selesai/tervalidasi. Scope platform/MFA/dukungan dan keputusan keamanan belum ditetapkan; fitur belum diimplementasikan.**
- [x] DILEWATI / TERBLOKIR — Izin dukungan dari Pemilik: baca saja, maks 24 jam, cakupan modul, kedaluwarsa otomatis, terlihat di halaman Keamanan (CORE-045) **Penutupan administratif, BUKAN selesai/tervalidasi. Scope platform/MFA/dukungan dan keputusan keamanan belum ditetapkan; fitur belum diimplementasikan.**
- [x] Worker menghapus draft active/held/cancelled tidak aktif >30 hari, mempertahankan completed, audit expired, batch 500/SKIP LOCKED dan uji row terkunci/idempotency cleanup (CORE-043)

### 2.13 Landing page
- [x] Setup `landing/` statis terpisah dari bundle aplikasi: Vite workspace, HTML multipage, build ke `landing/dist`, dev port 5174 (§16.8)
- [x] Integrasi website satu origin: dev utama `app` port 5173 membuka landing di `/`, login `/masuk`, dashboard `/dashboard`. Build `app/dist` menggabungkan HTML/aset landing dan entry `admin.html`; preview memetakan route aplikasi. Bundle landing tetap terpisah dari React (§16.8)
- [x] Design tokens biru/putih/abu mengikuti arahan pengguna dan referensi Bigspring; system sans + mono tanpa font eksternal. Pasangan teks utama 4,82–13,65:1, tombol putih/biru 5,17:1, batas input 3,19:1. Menggantikan usulan Kunyit/Plus Jakarta Sans (§19.2)
- [x] LP-001 Navbar sticky + menu mobile dialog native, fokus modal/ESC/return focus, skip link dan fallback navigasi tanpa JS tersedia
- [x] DILEWATI / TERBLOKIR — LP-002 Hero terpusat mengikuti Bigspring dengan ilustrasi logo pengguna tersedia. Pratinjau dashboard/stok/laporan merender komponen aplikasi asli dengan label Data demo; tangkapan layar raster build live masih perlu browser **Penutupan administratif, BUKAN selesai/tervalidasi. Landing page dikelola pengguna; provider/domain/perangkat/QA eksternal tidak dikerjakan agen, bukan bukti kelulusan.**
- [x] LP-003 Alur 4 langkah penjualan → stok → kas → laporan dengan data demo Toko Sumber Rejeki (§19.3)
- [x] LP-004 Tab jenis usaha ARIA, Arrow/Home/End, konten terbaca tanpa JS, CTA mengisi jenis usaha form tersedia
- [x] LP-005 Enam fitur dan bagian editorial bergantian dashboard/persediaan/laporan mengikuti Bigspring; pratinjau HTML dari komponen produk asli
- [x] LP-006 UI stepper 5 langkah dengan tombol Sebelumnya/Berikutnya, keyboard, aria-live dan fallback tanpa JS. Contoh keranjang Rp175.000 dan kas akhir sesuai §19.3; tangkapan layar tiap langkah belum
- [x] LP-007 Perangkat, akses peran/log aktivitas + daftar Belum didukung tersedia; tanpa klaim perangkat fisik teruji
- [x] LP-008 Blok konsultasi tanpa harga dengan CTA Diskusikan Kebutuhan
- [x] LP-009 FAQ akordion native details/summary, keyboard, semua jawaban tersedia di HTML
- [x] LP-010 CTA akhir, form, footer dan tautan informasi tersedia; privasi/syarat masih dokumen pengembangan yang menunggu validasi sebelum publikasi
- [x] LP-011 Form demo: validasi blur/submit dan server, telepon E.164, consent terpisah, honeypot, state busy/error/success, timeout, input tetap saat gagal, Idempotency-Key. Server mengimplementasikan batas 5 pengiriman berkunci baru/peer/jam dan dedup 10 menit; integrasi DB belum terverifikasi
- [x] DILEWATI / TERBLOKIR — LP-011 Endpoint `POST /api/v1/leads`, migrasi 0060 `platform.leads`/limits dan outbox `lead.notification` tersedia. Tes integrasi concurrent retry/dedup/rate limit ditambahkan tetapi dilewati karena socket PostgreSQL dibatasi. Penerapan migrasi lokal dan pengiriman notifikasi via penyedia **(blok: TBK-10)** belum **Penutupan administratif, BUKAN selesai/tervalidasi. Landing page dikelola pengguna; provider/domain/perangkat/QA eksternal tidak dikerjakan agen, bukan bukti kelulusan.**
- [x] DILEWATI / TERBLOKIR — LP-012 Event lokal `storepos:analytics` dengan allowlist properti tanpa PII + tes payload tersedia/lulus; integrasi/izin penyedia analytics belum **(blok: penyedia analytics)** **Penutupan administratif, BUKAN selesai/tervalidasi. Landing page dikelola pengguna; provider/domain/perangkat/QA eksternal tidak dikerjakan agen, bukan bukti kelulusan.**
- [x] DILEWATI / TERBLOKIR — LP-013 Meta/OG, canonical/sitemap/robots bersyarat domain final, dimensi gambar, reduced motion dan fallback tanpa JS tersedia; domain final, Lighthouse ≥90, CLS ≤0,1 dan QA browser belum **Penutupan administratif, BUKAN selesai/tervalidasi. Landing page dikelola pengguna; provider/domain/perangkat/QA eksternal tidak dikerjakan agen, bukan bukti kelulusan.**
- [x] Animasi scroll IntersectionObserver muncul ulang saat section masuk viewport; reduced motion dan fallback tanpa JS, tanpa dependensi animasi tambahan
- [x] DILEWATI / TERBLOKIR — LP-015 Halaman `/perangkat.html` tersedia dengan status belum diverifikasi; pengujian model/OS/browser/tanggal perangkat fisik belum dilakukan **Penutupan administratif, BUKAN selesai/tervalidasi. Landing page dikelola pengguna; provider/domain/perangkat/QA eksternal tidak dikerjakan agen, bukan bukti kelulusan.**
- [x] Review copy fitur: tanpa angka pelanggan/testimoni, klaim pembayaran manual, keterbatasan offline/split bill/transfer cabang, laba estimasi dan apotek pilot dinyatakan; tanpa placeholder [TBK] di beranda

Catatan 14 Oktober 2026: referensi screenshot Bigspring pengguna mengesampingkan layout/warna/motion usulan §19.2. Build landing dan unit validasi/analytics lulus. Chrome dan koneksi PostgreSQL dibatasi socket pada lingkungan pengerjaan; audit visual desktop/mobile, kirim lead end-to-end, tangkapan layar live dan Lighthouse tetap terbuka. Lihat `landing/README.md` untuk menjalankan, konfigurasi domain/proxy dan batas verifikasi.

### 2.14 Pengujian retail
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: E2E alur §8.2, §8.5, §8.6, §8.7, §8.8 di browser acuan termasuk scanner tersimulasi
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: Uji perangkat manual: printer & scanner acuan (CORE-022, RTL-004, RTL-005)
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: Uji usabilitas untuk M1 (≤ 25 detik) dan M3 (≥ 80%)

---

## Fase 3 — MVP Restoran

- [x] Backend restore area/meja If-Match: cabang aktif, area parent aktif, duplikat nama ditolak dan audit; pembacaan modifier include_archived/nonaktif khusus pengelola; snapshot nama stasiun tiket baru dan missing flag histori lama (RST-002/004/007). Tanpa fungsi tes baru

- [x] Backend riwayat order status open/completed/cancelled/all + tipe/periode lokal (histori wajib periode), cursor dan kontak tidak diisi pada list; master area/meja/stasiun include_archived hanya pengelola (RST-003/CORE-043). Tanpa fungsi tes baru

- [x] Backend POST open menerima kontak/antrean optional dengan validasi tipe dan batas ongkir; DELETE order/contact khusus Pemilik mengosongkan kontak/antrean dengan versi/audit tanpa PII. Retensi backup/legal belum (RST-001)
- [x] Backend snapshot category_id saat tambah item dipakai routing dapur; histori lama dibackfill bertanda legacy, master kategori baru tidak merutekan ulang item lama (RST-005). Tanpa fungsi tes baru

- [x] Backend edit header order dengan If-Match: note/channel/ongkir, nama/antrean takeaway dan kontak penerima delivery; detail hanya authenticated, audit tanpa PII kontak (RST-001/003). Field dapat diisi via POST open maupun PUT; endpoint erase kontak tersedia, tanpa fungsi tes baru
- [x] Backend bill preview rincian line snapshot/modifier/qty/harga/neto/tax/SC, label BELUM LUNAS; CANCELLED dikecualikan, NOT_SENT count dan payment_blocked (RST-010). Checkout belum

- [x] Backend PUT /session/business memvalidasi anggota aktif dan menyimpan bisnis sesi untuk EventSource tanpa header custom; audit dan SSE tutup bila konteks sesi berganti. Tanpa fungsi tes baru (CORE-004, RST-005)
- [x] Backend GET tiket dapur detail termasuk closed/cancelled, snapshot modifier/note/qty, status/current cancellation dan label bukan struk pembayaran. Frontend cetak/perangkat belum (RST-007)

- [x] Backend SSE invalidasi order/meja dan ringkasan READY per cabang, pemeriksaan akses ulang setiap polling; middleware ResponseController unwrap mendukung streaming. Tanpa fungsi testing baru; verifikasi stream manual/performa belum

- [x] Backend cek Habis saat penambahan qty item NOT_SENT dan pengiriman dapur; menurunkan qty/hapus tetap diperbolehkan (RST-013). Tanpa fungsi tes baru
- [x] Worker rekonsiliasi reserved_milli vs jumlah reservasi aktif dan reserved>on_hand dalam snapshot SQL tunggal, temuan/alert persisten (RST-013, NFR-012)
- [x] Backend DELETE routing kategori dengan cabang/If-Match/audit agar konfigurasi bisa dilepas sebelum arsip stasiun (RST-005)

- [x] Backend manual menu Habis per varian/cabang dengan versi/audit, daftar availability/on_hand/reserved/available, item baru sold_out ditolak; laporan stok/reorder memakai available untuk minimum (RST-013). Cek qty bertambah/kirim sold_out tersedia; checkout masih terbuka; tanpa fungsi tes baru

- [x] Backend reservasi menu track_stock saat send (lock saldo urut varian), pelepasan idempoten saat cancel line dan proteksi reserved dari retail/adjustment/opname yang mengurangi stok (§13.4, RST-013). Tidak menambah fungsi testing baru

- [x] Backend arsip area/meja dengan versi/audit, meja open ditolak dan area dengan meja aktif ditolak; create meja mengunci area agar tidak berlomba arsip (RST-002)
- [x] Backend edit/arsip stasiun (routing/tiket aktif harus dipindah/diselesaikan), daftar routing kategori dengan cursor/versi (RST-005). Tanpa fungsi tes baru

- [x] Backend konfigurasi/read SC per cabang/jenis order, rate/tax-base/version/audit; bill preview BELUM LUNAS memakai pricing snapshot item/modifier+pajak/SC/ongkir dan mengecualikan CANCELLED. Pembayaran tetap blocked (RST-009/010)
- [x] Backend cancel order open setelah item terkirim dibatalkan lewat approval, reason histori, close timestamp/audit; status meja diturunkan OCCUPIED/NEEDS_CLEANING/AVAILABLE dan aksi clean berizin/versi (RST-002/010). Tanpa fungsi tes baru

- [x] Backend laporan pembatalan restoran periode/cabang/pelaku: snapshot item/modifier, status asal/potensi waste, alasan/penyetuju/waktu lokal; CSV query sama, izin/modul/cabang dan audit ekspor (RST-008). Nilai menu bukan HPP/kerugian; tanpa fungsi testing baru

- [x] Backend pembatalan item dengan versi/alasan/approval sesuai status, snapshot status asal/pelaku/penyetuju dan audit; KDS menampilkan CANCELLED/alasan selama 24 jam pada order open (RST-008). Laporan JSON/CSV pembatalan tersedia; pelepasan reservasi saat cancel tersedia; WASTE fisik/config masih terbuka

- [x] Backend daftar tiket KDS aktif per stasiun/cabang: urutan waktu, snapshot modifier/catatan/jenis/meja, status item terkini/versi order/server_time, cursor dan no-store (RST-005). Poll API tersedia, SSE/UI belum

- [x] Backend edit/arsip grup modifier dan edit/nonaktif opsi dengan If-Match versi grup, tenant/modul/peran/audit; snapshot baris lama tidak diubah (RST-004). Fungsi testing baru tidak ditambah

- [x] Backend pemilihan modifier saat tambah baris: validasi group min/max/wajib, opsi aktif milik produk tanpa duplikat, snapshot nama/grup/harga, delta dan effective_unit_price tersedia. Kombinasi berbeda baris terpisah (RST-004). Tes khusus belum ditambah

- [x] Backend create/list grup modifier per produk dengan min/max pilihan dan opsi tambahan harga >=0, versi grup/audit/modul/izin. Penerapan/snapshot opsi baris dan edit/nonaktif belum (RST-004), tanpa fungsi tes baru

- [x] Backend add/edit/remove/list baris pesanan NOT_SENT dengan versi order, snapshot nama/harga/satuan server, qty/fractional dan catatan 140 karakter; tenant/cabang/audit enforced (RST-003). Tanpa fungsi tes baru, tes khusus fitur belum

- [x] Backend create/open/resume/get/list header pesanan restaurant lintas terminal satu cabang tanpa shift; sesi/modul/izin/cabang dan audit. Baris/edit/pembayaran belum (RST-001/003)

- [x] Backend list area termasuk area kosong + edit nama area/nama-kapasitas meja dengan If-Match, audit dan akses modul/peran/cabang. Arsip/status/pindah area belum; tanpa fungsi tes baru (RST-002)

- [x] Backend create area/create meja/list meja per area dengan cursor, kapasitas dan FK tenant/cabang; modul restaurant/izin/cabang enforced. Tidak menambah fungsi testing baru, tes fitur khusus belum (RST-002)

- [x] Backend header pesanan DINE_IN (meja wajib)/TAKEAWAY/DELIVERY_MANUAL (kanal/ongkir) tersedia. Penerima/antrean opsional tersedia melalui edit header; SC dan ongkir diterapkan saat pembayaran (B160) (RST-001)
- [x] Backend dan UI master area/meja/list/filter/grid tersedia; edit/arsip/pulihkan memakai versi dan audit, area dengan meja aktif serta meja dengan order terbuka ditolak 409; `/meja` untuk operasional dan `/meja/kelola` untuk Pemilik/Manajer (RST-002, §11.1). Tes API lifecycle lulus; belum diuji di browser
- [x] Backend satu pesanan open per meja (unique index + row lock); open ulang mengembalikan order_id/version existing tanpa mengubah header (RST-002). Tes fitur khusus belum ditambah
- [x] Backend pesanan OPEN lintas terminal dengan versi/konflik, checkout terikat shift pembayaran saat ini tersedia (RST-003, E14). Uji alur baru khusus tetap terbuka
- [x] Backend grup modifier wajib/opsional/min/max/harga/catatan, snapshot item dan penerusan checkout tersedia (RST-004). UI/tes khusus baru belum
- [x] Backend create/list stasiun per cabang dan mapping kategori→stasiun dengan If-Match/audit/FK tenant-cabang; kitchen read dan konfigurasi Pemilik/Manajer dibatasi modul/cabang (RST-005). Edit/arsip dan pemakaian routing saat kirim kini tersedia; tanpa fungsi testing baru
- [x] Backend POST order/send dengan Idempotency-Key+If-Match, NOT_SENT→SENT, tiket baru per stasiun/pengiriman, snapshot nama/modifier/qty/catatan dan routing kategori; missing route rollback, replay dan audit atomik (RST-005). Tidak membuat fungsi testing baru; uji khusus alur masih terbuka
- [x] Implementasi backend SSE refresh/heartbeat dan frontend EventSource/fallback polling 5 detik, reconnect online/visibility, abort scope lama, refresh ready_count grid meja tersedia (§16.4, B470). **Testing/load realtime DITUNDA, belum dijalankan/belum lulus; implementasi bukan klaim siap produksi.**
- [x] Layar dapur (KDS): teks besar, satu ketuk maju status, umur tiket, banner terputus + reconnect (RST-005, RST-006, E15). Diimplementasi di `/dapur` (B130); belum diuji di browser nyata maupun jam sibuk
- [x] Backend status SENT→IN_PROGRESS→READY (izin dapur), READY→SERVED (izin kasir), mundur satu langkah wajib alasan; If-Match order/audit/tenant-cabang-modul enforced (RST-006). Tidak menambah fungsi testing baru
- [x] Backend ready_count dan item_status_counts per pesanan tersedia untuk indikator Siap diantar (RST-006); frontend grid meja menampilkan jumlah item siap diantar. Belum diuji di browser
- [x] Backend detail tiket untuk template/cetak tersedia; frontend KDS memuat detail snapshot dan pratinjau/cetak CSS 80 mm (kompatibel penyusutan printer 58 mm). Validasi printer fisik tetap uji eksternal (RST-007)
- [x] Pembatalan item sesuai status + O + laporan item dibatalkan (RST-008). UI di halaman pesanan (form inline alasan + PIN atasan bila diperlukan); belum diuji di browser
- [x] Backend aturan SC/preview/checkout dan snapshot rate/tax-base tersedia per cabang/jenis pesanan (RST-009). Validasi pajak dan uji khusus masih terbuka
- [x] Backend bill BELUM LUNAS dan checkout blok NOT_SENT, CANCELLED dikecualikan; ongkir nonzero kini didukung saat checkout (B160) (RST-010). Frontend/tes khusus belum
- [x] Backend status meja derived dan endpoint clean versi/audit tersedia (RST-010). Frontend/tes khusus belum
- [x] Menu "Habis" manual + reservasi stok untuk menu `track_stock` (RST-013, §13.4). UI `/menu-habis` (hanya pemilik/manajer yang bisa mengubah); belum diuji di browser
- [x] Backend GET restaurant/reports/aged-orders: threshold 1..720 jam (default 4), usia server/cabang/jenis/meja/versi dan cursor, izin report.sales/modul/cabang (E12). UI/alert eksternal belum
- [x] DILEWATI / TERBLOKIR — Keputusan: pindah meja masuk MVP atau tidak (RST-011) **(blok: TBK-09)** **Penutupan administratif, BUKAN selesai/tervalidasi. Menunggu keputusan cakupan produk TBK-09; perpindahan meja belum dibuat.**
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: Uji layar dapur jam sibuk simulasi ≥ 30 tiket/jam (§22.2)
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: E2E alur §8.3

---

## Fase 4 — Pilot Apotek

Backend fondasi batch/obat/FEFO preview mulai tersedia (B90); task induk di bawah tetap terbuka karena validasi/regulasi/alur akhir dan integrasi belum lengkap.

- [x] Data obat: zat aktif, sediaan, kekuatan, produsen, izin edar (teks), golongan, wajib resep, pembatasan khusus (APT-001) Backend tersedia; golongan berupa teks, daftar default menunggu [VAL].
- [x] Satuan kemasan Dus/Strip/Tablet, tanpa pecahan default (APT-002) Backend: penjualan satuan kemasan dikonversi ke satuan dasar sebelum alokasi batch; pecahan diblokir.
- [x] Batch + ED wajib pada penerimaan; ED bulan/tahun → akhir bulan; tolak ED lampau atau karantina (APT-003, E18) Backend tersedia (migrasi 0056).
- [x] Saldo per (cabang, varian, batch, status); stok negatif dilarang (APT-004) Backend tersedia.
- [x] Pratinjau alokasi FEFO di kasir + alokasi ulang di dalam lock saat finalisasi (APT-005, E16). `FefoPreview` tampil per baris keranjang, label ED/shortage; finalisasi mengunci dan menghitung ulang server. Belum diuji di browser
- [x] Override batch manual oleh Apoteker dengan alasan (APT-005) Backend: hanya apoteker (login/PIN), alasan wajib, batch sellable cukup.
- [x] Blokir batch kedaluwarsa berdasarkan tanggal lokal cabang, BATCH_EXPIRED (APT-006) Backend: tanggal lokal cabang, ED dianggap tidak boleh dijual.
- [x] Daftar "Mendekati ED" + label di kasir, ambang default 90 hari (APT-007). Filter/label tersedia di `/batch` dan `FefoPreview`; belum diuji di browser
- [x] DILEWATI / TERBLOKIR — Referensi resep (APT-008): akses baca khusus Apoteker via endpoint pharmacy/sales/{id}/prescription dengan tenant/cabang/audit/no-store + UI terbatas selesai implementasi B590, PRESCRIPTION_REFERENCE_REQUIRED tersedia. Field minimum dan validasi regulasi masih **(blok: VAL)**; testing DITUNDA, bukan lulus. **Penutupan administratif, BUKAN selesai/tervalidasi. Validasi field minimum/regulasi/apoteker eksternal dilewati; tidak boleh mengklaim patuh atau mengaktifkan pilot berdasarkan centang ini.**
- [x] Verifikasi oleh Apoteker (login atau PIN) (APT-009) Backend: apoteker login atau email+PIN apoteker.
- [x] Retur ke karantina + pelepasan QUARANTINE_RELEASE oleh Apoteker (APT-010) Backend: retur restock masuk QUARANTINE batch asal; pelepasan oleh apoteker.
- [x] Penelusuran batch: penerimaan → pergerakan → penjualan → retur (APT-011) Backend: GET /pharmacy/batches/{id}/trace.
- [x] Write-off batch → DISPOSED (APT-012) Backend tersedia.
- [x] Stok opname per batch (APT-013) Backend: hitung per batch/status dengan COUNT_ADJUSTMENT; alur sesi opname multi-batch belum.
- [x] Laporan stok per batch (CORE-039). `GET /reports/stock/batches` (JSON/CSV teraudit): saldo per batch/status, EXPIRED diturunkan dari tanggal lokal cabang, filter mendekati ED/kedaluwarsa/karantina, pergerakan periode, biaya rata-rata tertimbang hanya untuk peran berizin. Sinkron (maks 100.000 baris), belum lewat antrean ekspor latar
- [x] Uji FEFO/ED termasuk batas tanggal dan zona waktu (§22.2). `pharmacy_test.go` + `pharmacy_boundary_test.go`: ED = hari ini ditolak, ED besok lolos, tanggal lokal cabang (UTC+14 vs UTC-11) menentukan, bukan tanggal server
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: E2E alur §8.4
- [x] DILEWATI / TERBLOKIR — Dokumen validasi apoteker/regulasi ditandatangani sebelum pilot **(blok: TBK-07)** **Penutupan administratif, BUKAN selesai/tervalidasi. Validasi field minimum/regulasi/apoteker eksternal dilewati; tidak boleh mengklaim patuh atau mengaktifkan pilot berdasarkan centang ini.**

---

## Kriteria Kesiapan Rilis (§22.2)

- [ ] Semua kebutuhan Must MVP segmen lulus AC
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: Uji isolasi tenant & matriks izin 100% lulus
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: Uji penetrasi eksternal tanpa temuan kritis/tinggi terbuka
- [ ] Vektor uji perhitungan identik di Go dan React
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: Uji konkurensi stok & nomor struk lulus
- [x] DITUNDA atas permintaan pengguna — BELUM DIJALANKAN/BELUM LULUS: Uji restore + RTO tercapai; jadwalkan restore bulanan (NFR-008, NFR-009)
- [ ] Rekonsiliasi invarian §15 = 0 selama pilot
- [ ] Runbook insiden: DB gagal, deploy gagal, kebocoran data (§16.7)
- [ ] Kebijakan privasi & syarat layanan terbit
- [ ] Daftar perangkat teruji dipublikasikan
- [ ] Retail: pilot ≥ 2 minggu di ≥ 3 toko, M1 & M3 tercapai
- [ ] Restoran: batasan (split bill, printer dapur) tercantum di LP
- [ ] Apotek: label "pilot" di LP

---

## Fase 5 — Lanjutan (backlog, prioritas pasca-pilot)

- [ ] Split bill, gabung pesanan, pindah meja (RST-011)
- [ ] Bridge lokal printer / laci kas / printer dapur (TBK-03)
- [ ] Pembayaran terintegrasi QRIS/kartu + webhook + rekonsiliasi (CORE-020)
- [ ] Harga grosir bertingkat (RTL-007), barcode timbangan (RTL-008)
- [ ] Purchase Order (CORE-034), transfer stok antar cabang, harga per cabang
- [ ] Resep/BOM bahan baku (RST-012)
- [ ] Mode offline terbatas retail (§17.5)
- [ ] Kirim struk via WhatsApp/email
- [ ] Peran kustom, kombinasi modul, MFA Pemilik
- [ ] Halaman landing per jenis usaha (LP-014), laporan regulasi apotek (APT-014)
