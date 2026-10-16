# Jenis usaha dan penguncian — CORE-002 / §5.3

GET /settings/business-type: business_type, enabled_modules, org_version sebagai version, locked dan first_sale_completed_at.
PUT endpoint sama, If-Match dan `{business_type:"retail|restaurant|pharmacy"}`. Owner/business.manage saja, mode valid; modul diturunkan dari jenis usaha (tidak dikirim bebas oleh klien). Versi stale atau sudah locked ditolak 409. Perubahan teraudit.

Finalisasi pertama mengisi first_sale_completed_at di awal transaksi; seluruh kegagalan rollback nilai tersebut. Perubahan jenis usaha dan finalisasi pertama diserialkan pada row bisnis. Penjualan berikutnya tidak menimpa waktu pertama. Migrasi 0032 backfill waktu MIN completed_at termasuk transaksi yang kemudian voided karena tetap merupakan transaksi final pertama. Trigger DB melarang perubahan jenis/modul atau mengosongkan flag setelah locked.

Histori backfill tidak dibatalkan oleh down migration. Seed pengembangan yang mencoba mengganti jenis/modules pada bisnis dengan penjualan kini dapat ditolak trigger; jangan menggunakan seed sebagai reset bisnis operasional.

Belum: onboarding/create business, kombinasi modul/peran custom, batas paket, dan pemasangan MODULE_DISABLED pada endpoint khusus restoran/apotek. Endpoint finalisasi generik belum implementasi batch/FEFO dan pesanan restoran; mengganti jenis usaha tidak berarti modul tersebut siap dipakai produksi. Frontend tidak diubah.
