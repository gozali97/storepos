# Pembacaan audit — CORE-042

GET /api/v1/audit-logs?from=RFC3339&until=RFC3339&branch_id=UUID&actor_user_id=UUID&action=...&entity_type=...&limit=100&before=UUID

from/until wajib, maksimal 366 hari, akhir eksklusif. Filter lainnya opsional. limit 1..100. Cursor memakai pasangan occurred_at/id sehingga timestamp sama tidak menggandakan/menghilangkan item. Cursor harus ada dalam cakupan tenant, cabang dan filter yang sama.

Izin audit.view: Owner dapat melihat semua event bisnis termasuk branch_id NULL. Manager hanya event cabang yang ditugaskan; tanpa penugasan tidak mendapatkan event. Event platform business_id NULL tidak pernah muncul pada endpoint tenant ini. CSV/export audit belum disediakan. Respons Cache-Control:no-store.

Output memuat pelaku, penyetuju, cabang/terminal, waktu UTC, tindakan/entitas, before/after, alasan dan IP/user-agent jika sudah dicatat. Field yang belum ditulis tetap null. Tidak ada endpoint mutasi audit.

Login berhasil/gagal/rate-limited dan logout kini diaudit sebagai event global business_id NULL. Password/email/token tidak ditulis. Login/logout mengubah sesi dan menulis audit dalam transaksi yang sama; kegagalan audit mencegah keberhasilan perubahan sesi. Metadata IP peer (tanpa mempercayai X-Forwarded-For) dan user-agent maksimal 512 karakter diteruskan melalui context pada semua audit request terautentikasi. Audit worker non-HTTP tetap tanpa IP/user-agent.

CORE-042 belum seluruhnya selesai: perubahan peran/penugasan, pencetakan ulang, dukungan platform dan farmasi masih mengikuti implementasi modul. Payload login tidak valid/field kosong belum dicatat sebagai auth.login_failed. Login gagal karena error DB bukan salah kredensial. IP/user-agent dapat menjadi data pribadi; akses dibatasi audit.view dan perlu kebijakan retensi legal. REVOKE UPDATE/DELETE role aplikasi dan retensi >=2 tahun masih memerlukan role deploy/kebijakan retensi; trigger append-only sudah ada, tetapi itu tidak menggantikan validasi hak DB aplikasi.

Frontend terpisah ditangani agent lain; tidak ada file app/landing yang diubah pada pekerjaan ini.
