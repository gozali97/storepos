# Detail tiket dapur — RST-007

GET /restaurant/kitchen/tickets/{id}, sesi/tenant/modul restaurant, izin kitchen.update_status atau sale.create dan cabang ditugaskan. Mengembalikan snapshot tiap item saat kirim (nama/varian/qty/satuan/modifier/catatan), jenis/meja tiket, waktu, stasiun saat ini, status order/version, status pembatalan terkini dan label TIKET DAPUR — BUKAN STRUK PEMBAYARAN.

Detail dapat dibaca setelah item served/order closed agar cetak ulang/penelusuran tetap mungkin; daftar KDS aktif tetap hanya tiket aktif. Share lock order menjaga status/cancellation konsisten dalam detail. Stasiun name berasal master terkini, bukan snapshot nama stasiun. Tidak ada harga bayar atau HPP di payload.

Frontend template/dialog cetak dan pengujian printer belum tersedia. Tidak ada audit cetak tiket dapur terpisah, endpoint ini read-only bukan bukti cetak. Tidak menambah fungsi testing baru; build/vet/suite lama lulus. Tidak mengubah frontend pada batch ini.
