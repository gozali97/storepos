# Ekspor latar — CORE-044

POST /api/v1/reports/{payments|sales|corrections|stock|products}/exports memakai query parameter yang sama dengan laporan sinkron (branch_id/from/to/user_id/group_by/filter). Sesi, tenant, izin baca laporan dan CSRF wajib. Hasil 202 `{export_id,status:"pending"}`.

GET /api/v1/report-exports/{id}: status/row_count/error_code/expires_at. Tambahkan download=true untuk CSV completed. Hanya peminta pada tenant yang sama, izin laporan/cabang dan izin biaya/laba yang diperlukan artifact diperiksa ulang. Pengguna dinonaktifkan atau izin dicabut tidak dapat mengunduh artifact lama. Unduhan no-store, tanpa URL publik.

storepos worker polling 5 detik, claim FOR UPDATE SKIP LOCKED. Claim/query/artifact/audit commit bersama sehingga crash mengembalikan pekerjaan pending; query memakai kalkulasi reporting yang sama tetapi tidak dibatasi 100.001 hasil. CSV menggunakan nama field DTO JSON dan nilai yang sama (susunan/header dapat berbeda dari CSV sinkron). Nilai NULL kosong; teks bebas dilindungi dari formula spreadsheet, angka negatif tetap negatif. Ekspor tercatat audit saat selesai.

Artifact disimpan di PostgreSQL dengan batas 50 MiB; hasil yang melewati batas gagal eksplisit EXPORT_ARTIFACT_TOO_LARGE. Retensi 7 hari, worker menghapus artifact dan menandai expired. Database metadata tetap ada. Tidak ada object storage atau streaming query; laporan masih dimuat ke memori, sehingga 50 MiB membatasi artifact saja, bukan seluruh penggunaan memori. Uji serializer 100.005 baris tersedia; uji query end-to-end DB >100.000 hasil dan benchmark memori belum.

Kegagalan izin ditandai failed/EXPORT_PERMISSION_REVOKED. Error DB/timeout rollback dan dicoba ulang polling berikutnya, belum ada backoff/max-attempt. Runbook harus memonitor log report export failed. Worker rekonsiliasi tetap berjalan sekali per tanggal UTC menggunakan claim harian; polling bertambah tidak menggandakan rekonsiliasi.
