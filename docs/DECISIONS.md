# Keputusan & Asumsi Kerja

Dokumen ini mencatat nilai default yang dipakai sampai keputusan resmi diambil pemilik produk/validator. Setiap entri bertanda **(default)** harus dikonfirmasi sebelum rilis.

## TBK yang memblokir desain

| ID | Keputusan default | Alasan |
| --- | --- | --- |
| TBK-01 Paket & harga | Belum ditetapkan; landing page pakai pola konsultasi (LP-008) | §19.11 |
| TBK-03 Bridge printer | **Tidak** di MVP; cetak via dialog browser (CORE-022) | §17.3 |
| TBK-04 Stok negatif retail | **Dilarang** secara default; opsi di pengaturan bisnis | §13.5, risiko §21.1 |
| TBK-05 Pembulatan tunai | **Aktif** ke kelipatan Rp100; dicatat sebagai `rounding_adjustment` | §13.8 langkah 8 |
| TBK-06 Nomor struk tanpa celah | **Ya** per cabang per hari dengan counter row-lock (CORE-046) | §13.12 |
| TBK-08 Lokasi data | **Belum diputuskan**; placeholder di LP-007 | §16.7 |
| TBK-09 Pindah meja MVP | **Tidak** di MVP; split bill fase berikutnya | §11, RST-011 |
| TBK-10 Notifikasi lead | Email transaksional via penyedia yang akan dipilih; outbox siap | §19.13 |

## Nilai default operasional (§23 no. 10–12)

| Parameter | Default | Dapat diubah |
| --- | --- | --- |
| Batas diskon Kasir | 10% per baris / 10% per transaksi | Pemilik, per peran |
| Ambang selisih kas untuk O | Rp10.000 absolut | Pemilik |
| Ambang kas keluar untuk O | Rp500.000 | Pemilik |
| Batas hari retur | Tidak dibatasi | Pemilik |
| HPP | Moving average per varian per cabang | — |
| Manajer lihat laporan laba | **Tidak** di MVP, hanya Pemilik | Fase berikutnya |
| Stok negatif retail | Dilarang | Pengaturan |

## Modul apotek yang menunggu validator (§12)

Daftar klasifikasi obat, field minimum referensi resep, aturan tanggal ED (inklusif vs eksklusif), kebijakan pelepasan karantina, serta produk yang diblokir dari penjualan biasa **belum diisi**. Modul apotek dibangun sebagai kerangka; nilai konfigurasi kosong sampai validator menandatangani (TBK-07, APT-014).

Default konservatif yang dipakai sampai validasi:

- ED dianggap hari pertama tidak boleh dijual (APT-006).
- ED bulan/tahun disimpan sebagai tanggal terakhir bulan tersebut (APT-003).
- Retur selalu ke karantina, pelepasan hanya oleh Apoteker (APT-010).

## Zona waktu & lokalisasi

- Zona waktu disimpan per cabang; semua angka laporan memakai tanggal lokal cabang (§13.10).
- Format uang: `Rp 1.234.567` (NFR-017).
- Format tanggal: `DD/MM/YYYY`.
- Bahasa UI: Bahasa Indonesia.

## Perangkat acuan (default sebelum uji nyata)

| Jenis | Model acuan sementara |
| --- | --- |
| Printer thermal 58 mm | — (diisi setelah uji) |
| Printer thermal 80 mm | — |
| Scanner HID USB | — |
| Scanner HID Bluetooth | — |
| Tablet Android | Chrome versi terbaru |
| iPad | Safari versi terbaru |
| PC | Windows 10/11 + Chrome/Edge |

## Catatan

- Semua pilihan di dokumen ini adalah **asumsi kerja**. PR yang mengubah nilai di sini harus menyebut sumber keputusan.
- Tidak ada klaim di landing page yang melebihi daftar MVP sebelum fitur lulus AC.
