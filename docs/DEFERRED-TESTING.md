# Testing ditunda atas permintaan pengguna

Status `[x] DITUNDA` berarti tidak dikerjakan dulu, bukan lulus. Tidak boleh digunakan sebagai bukti kesiapan rilis. Hasil pengujian lama tetap seperti tercatat di batch masing-masing.

- [x] **DITUNDA / BELUM LULUS** — Fase 1 — Fondasi: Uji browser desktop/tablet/mobile dan login dengan PostgreSQL nyata; lingkungan agen menolak socket Vite/PostgreSQL dan peluncuran Chromium
- [x] **DITUNDA / BELUM LULUS** — Fase 2 — MVP Retail: Uji unit HPP dan aturan stok ≥ 90% (NFR-020)
- [x] **DITUNDA / BELUM LULUS** — Fase 2 — MVP Retail: Uji laporan pembayaran vs tunai neto shift, tanggal retur lokal, dan laporan penjualan satu hari vs 3 shift tertutup serta batas tengah malam tersedia (CORE-036). Skenario lintas-hari/tax/diskon/refund gabungan seluruh shift belum
- [x] **DITUNDA / BELUM LULUS** — Fase 2 — MVP Retail: Uji query laporan penjualan PostgreSQL lokal 5.000 transaksi/hari ×5 lulus ≤3 detik (12–845 ms dengan -race). Target seluruh laporan/API, dataset multitenant besar dan beban konkuren belum divalidasi (NFR-004)
- [x] **DITUNDA / BELUM LULUS** — Fase 2 — MVP Retail: E2E alur §8.2, §8.5, §8.6, §8.7, §8.8 di browser acuan termasuk scanner tersimulasi
- [x] **DITUNDA / BELUM LULUS** — Fase 2 — MVP Retail: Uji perangkat manual: printer & scanner acuan (CORE-022, RTL-004, RTL-005)
- [x] **DITUNDA / BELUM LULUS** — Fase 2 — MVP Retail: Uji usabilitas untuk M1 (≤ 25 detik) dan M3 (≥ 80%)
- [x] **DITUNDA / BELUM LULUS** — Fase 3 — MVP Restoran: Uji layar dapur jam sibuk simulasi ≥ 30 tiket/jam (§22.2)
- [x] **DITUNDA / BELUM LULUS** — Fase 3 — MVP Restoran: E2E alur §8.3
- [x] **DITUNDA / BELUM LULUS** — Fase 4 — Pilot Apotek: E2E alur §8.4
- [x] **DITUNDA / BELUM LULUS** — Kriteria Kesiapan Rilis (§22.2): Uji isolasi tenant & matriks izin 100% lulus
- [x] **DITUNDA / BELUM LULUS** — Kriteria Kesiapan Rilis (§22.2): Uji penetrasi eksternal tanpa temuan kritis/tinggi terbuka
- [x] **DITUNDA / BELUM LULUS** — Kriteria Kesiapan Rilis (§22.2): Uji konkurensi stok & nomor struk lulus
- [x] **DITUNDA / BELUM LULUS** — Kriteria Kesiapan Rilis (§22.2): Uji restore + RTO tercapai; jadwalkan restore bulanan (NFR-008, NFR-009)

Testing baru B430 juga tidak dijalankan. Checklist fitur implementasi yang belum selesai tetap terbuka. Validasi regulasi, deployment, dan perangkat eksternal tidak dinyatakan selesai.
