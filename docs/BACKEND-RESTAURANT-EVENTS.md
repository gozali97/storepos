# SSE dan status restoran

GET /restaurant/events?branch_id=UUID dengan cookie sesi, konteks bisnis dan izin kitchen.update_status atau sale.create. MIME text/event-stream, event refresh berisi branch_id saja, retry 2000 ms, heartbeat komentar. Frontend reload endpoint tables/tickets/orders yang sesuai izin; tidak menganggap SSE sebagai snapshot transaksi.

Stream mempoll DB setiap 2 detik, revalidasi token sesi/user/membership/modul/cabang dan tutup bila dicabut. Snapshot hash berisi order open ID/version/status dan meja version/occupied/needs_cleaning. Ditutup setelah 50 detik agar reconnect sesuai timeout HTTP server. Tidak ada persistent event ID/Last-Event-ID; setiap koneksi awal memberi refresh penuh. Refresh konfigurasi availability/stasiun belum termasuk hash.

Browser EventSource tidak dapat mengirim X-Business-ID custom; gunakan fetch streaming dengan header dan abort/reconnect, atau panggil PUT /api/v1/session/business {business_id} dengan sesi/CSRF sebelum membuka EventSource; endpoint memvalidasi keanggotaan aktif dan mencatat audit. Stream tanpa X-Business-ID tutup jika bisnis sesi berpindah; semua tab berbagi cookie sesi, jadi konteks bisnis juga dibagi. Jangan menaruh session token di URL. Frontend tidak diubah.

GET /restaurant/order-status?branch_id=UUID memberi item_status_counts/ready_count/order version/type/table. X-Poll-Interval:5 untuk fallback. Maks 1000, truncated eksplisit; frontend memakai list orders paginated untuk cakupan lebih besar. Kitchen-only dapat membaca ringkasan via endpoint ini tetapi bukan detail order yang butuh sale.create.

Implementasi sederhana DB polling per koneksi, bukan broker/event-outbox; uji stream HTTP langsung/performa/reconnect/revokasi bersamaan belum dilakukan. Tidak membuat fungsi testing baru; build/vet/suite lama lulus. Cegah buffering reverse proxy (X-Accel-Buffering:no) dan jangan cache SSE.
