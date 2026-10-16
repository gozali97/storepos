# Batch 30 — cakupan pajak produk/kategori

POST /tax-rules {name,rate_bps,mode} membuat aturan khusus (bukan default legacy bisnis). GET /tax-rules?after=UUID termasuk arsip, GET /tax-rules/{id} detail, PUT detail dengan If-Match mengganti nama/tarif/mode, DELETE dengan If-Match mengarsipkan hanya jika belum ditugaskan. Semua business.manage/Owner, tenant/CSRF dan audit. Tarif 0..10000 bps, mode inclusive/exclusive.

PUT /products/{id}/tax-rule atau /categories/{id}/tax-rule, If-Match versi MASTER, {tax_rule_id:UUID|null}. FK mencegah lintas tenant, arsip ditolak, versi master naik. Null melepaskan assignment. Rate 0 khusus digunakan untuk pengecualian eksplisit; null berarti fallback, bukan bebas pajak.

Resolver finalisasi dan bill restoran: aturan produk menang atas kategori, lalu default /settings/tax bisnis. Setiap sale_line snapshot nama/rate/mode; laporan penjualan mengecek mode baris, bukan mode header global. Histori lama line tax diisi dari header sales oleh migrasi 0052. Header sales.tax_* tetap default bisnis, bukan ringkasan seluruh scoped rules: klien mixed-tax harus memakai snapshot baris. Detail API baris belum menampilkan ketiga field pajak baru, perlu dilengkapi.

Belum: aturan per cabang, pajak bertumpuk satu item, GET assignment khusus, exposure snapshot pajak detail struk, checkout restoran, dan uji khusus mixed tax/konkurensi. Arsip rule share/update lock menserialkan assignment vs archive; pengeditan master tidak merekonstruksi histori. Tidak menambah fungsi tes baru; build/vet/suite lama -race lulus, frontend tidak diubah. Validasi konsultan pajak tetap syarat rilis.
