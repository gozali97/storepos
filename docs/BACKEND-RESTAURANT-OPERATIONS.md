# Operasi restoran — RST-002/009/010

- PUT /restaurant/service-charge, If-Match 0 baru/versi sekarang, {branch_id,order_type,rate_bps,include_in_tax_base}; Owner/Manager. Tarif 0..10000 bps. GET endpoint sama dengan branch_id untuk membaca aturan.
- GET /restaurant/orders/{id}/bill: pratinjau BELUM LUNAS, harga/modifier snapshot item non-CANCELLED dihitung memakai pricing engine + tax aktif + SC cabang/jenis + ongkir terpisah. Jumlah NOT_SENT tersedia. payment_blocked selalu true sampai endpoint pembayaran dibuat. Belum diskon restoran atau aturan pajak kategori.
- POST /restaurant/orders/{id}/cancel, If-Match, {reason}: menutup order setelah tidak ada item SENT/IN_PROGRESS/READY/SERVED. Item terkirim harus dibatalkan satu-satu lewat approval. Jika pernah terkirim, alasan order wajib. NOT_SENT ditandai CANCELLED, closed_at tersimpan; histori tidak dihapus.
- GET /restaurant/tables sekarang status OCCUPIED jika open, NEEDS_CLEANING jika order closed_at setelah cleaned_at, AVAILABLE lainnya. POST /restaurant/tables/{id}/clean, If-Match: sale.create, cabang sama, tidak ada order open. Meja dirty tidak menerima order baru.

Migrasi 0045; audit konfigurasi/cancel/clean. FRONTEND tidak diubah, tidak menambah fungsi testing baru. Build/vet/suite lama lulus, belum uji khusus operasi ini.

Pajak/SC pada bill memakai konfigurasi sekarang, belum immutable sampai pembayaran final. Ongkir belum memiliki validasi overflow agregat/pricing menyeluruh; jangan menerima pembayaran berdasar preview ini. SC final, reservasi/reversal stok, checkout, SSE, arsip master dan delivery recipient masih terbuka. Label BELUM LUNAS bukan struk lunas.
