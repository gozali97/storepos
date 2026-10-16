# B470 — 20 perbaikan operasional restoran

**Testing ditunda atas permintaan pengguna. Tidak ada klaim lulus/browser siap produksi.**

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

Berkas utama: `useRestaurantFeed.js`, `KitchenPage.jsx`, `TablesPage.jsx`, `TableManagementPage.jsx`, `KitchenTicketPrint.jsx`, `app.css`. Tidak ada perubahan landing, provider eksternal, atau skema/data backend.

Batas: hook membatasi 100 halaman (maks 10.000 item) dan memberi error eksplisit; polling tetap dilakukan walau SSE hidup untuk fallback. Grid memakai dua feed/stream (meja dan order status), belum diukur beban. Native EventSource memakai bisnis aktif pada sesi karena tidak bisa mengirim X-Business-ID; API data memakai header bisnis dari client. Printer 58/80mm, fokus dialog, reconnect/offline dan perpindahan cakupan perlu diuji di browser akhir.

- [x] **DITUNDA: testing/vet/build aplikasi/browser, belum dijalankan/belum lulus.**
