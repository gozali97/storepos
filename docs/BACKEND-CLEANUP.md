# Cleanup worker — CORE-043 / CORE-017

storepos worker menjalankan cleanup setiap siklus (poll 5 detik setelah pekerjaan sebelumnya selesai), maksimal 500 draft dan 500 kunci per transaksi. Migrasi 0033 menambahkan indeks kandidat draft.

Draft active/held/cancelled dengan updated_at lebih tua 30 hari dan completed_sale_id NULL dihapus. Draft completed, transaksi final, pergerakan stok dan audit tidak dihapus. Pemilihan FOR UPDATE SKIP LOCKED melewati draft yang sedang dimutasi. Audit draft.expired menyimpan ID/cabang/terminal/alasan dengan actor NULL; payload/catatan draft tidak disalin. Penghapusan dan audit atomik.

Idempotency key hanya dibersihkan bila expires_at lampau DAN created_at lebih tua 72 jam. Lock yang sedang dipakai request dilewati. Setelah retensi berakhir, kunci dapat dipakai lagi sebagai operasi baru; klien jangan retry aksi keuangan berhari-hari tanpa memeriksa transaksi asal.

Cleanup tidak menjalankan retensi audit/penjualan atau backup. Pengguna kehilangan draft yang tidak aktif >30 hari; kebijakan ini perlu ditampilkan oleh agent frontend. Tidak ada endpoint manual destruktif. Pembersihan berlangsung hanya jika worker aktif; error masuk structured logs. Uji draft fresh/completed/locked dan retensi minimum kunci tersedia.
