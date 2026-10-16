# Edit header dan rincian bill restoran

PUT /restaurant/orders/{id}, If-Match versi order, mengganti penuh `{note,channel,delivery_fee_cents,contact_name,contact_phone,queue_label}` saat order OPEN. Cabang/table/type tidak dipindahkan. DINE_IN tidak menerima nama kontak/antrean; TAKEAWAY dapat nama/antrean; DELIVERY_MANUAL dapat kanal/nama/HP/antrean/ongkir. Ongkir 0..Rp100 juta, string sen; note 200, name/channel 100, antrean 40 karakter, phone 32 byte dengan karakter nomor umum. Missing fields mengganti ke kosong/0, jadi baca detail sebelum edit. Audit hanya ID/version, tidak menyalin kontak. Detail order memuat kontak; list header order tidak mengisi kontak demi data minimal.

GET /orders/{id}/bill kini memuat lines: snapshot nama/varian/satuan/note/modifiers, qty, effective unit price, line_total, tax dan SC. Harga tetap snapshot saat item ditambah, konfigurasi pajak/SC sekarang untuk preview. CANCELLED dikecualikan, NOT_SENT count disertakan. Label BELUM LUNAS, payment_blocked=true. Tidak menerima pembayaran via preview.

Migrasi 0049, frontend tidak diubah. Tidak menambah fungsi testing baru; build/vet/suite lama lulus. Belum: delete/anonymize penerima delivery, retensi kontak, populasi contact pada POST open (gunakan PUT), diskon restoran, checkout final/SC/ongkir snapshot. Data kontak tidak tampil public receipt atau tiket dapur.
