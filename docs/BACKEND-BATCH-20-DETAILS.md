# Batch 20 — 10 task detail/riwayat/keamanan

1. GET /settings/tax: rule aktif {id,name,rate_bps,mode}, rule null bila kosong; business.manage.
2. GET /categories/archived?after=UUID: 100/next_cursor, product.manage.
3. GET /variants/{id}: variant DTO, status dan product_id; product.view, tenant. Tidak harga beli/HPP. Barcode array dari repository detail belum dimuat.
4. GET /purchase-units/{id}: variant/name/factor/sale_price/archived/factor_locked/version; product.view.
5. GET /branches?after=UUID: Owner semua tenant, Manager cabang ditugaskan, branch.view; status termasuk arsip.
6. GET /me/sessions?after=UUID: sesi akun sendiri aktif/belum expired, ID/current/IP/user-agent/waktu, tanpa token/hash, no-store.
7. POST /me/sessions/revoke-others: sesi+CSRF; cabut seluruh sesi akun lain device (lintas bisnis), pertahankan sesi sekarang, audit global atomik. Tidak perlu business ID.
8. GET /report-exports?after=UUID: peminta sendiri tenant/cabang saat ini; metadata tidak artifact. GET detail/download lama masih wajib revalidasi seluruh izin/biaya/laba. Tidak semua jenis report permission dicek ulang pada list metadata.
9. GET /stock/counts?branch_id=UUID&status=draft|posted&after=UUID: stock.count_input, penugasan cabang, 100/next_cursor.
10. GET /expenses/{id}: expense.manage dan cabang; ID/tanggal/kategori/amount/metode/note/shift/approval.

Tidak memerlukan migrasi. Default pagination 100. Tidak membuat fungsi tes baru; build/vet/suite backend lama -race lulus. Uji langsung seluruh endpoint/konkurensi/revokasi saat response belum; frontend tidak diubah. Endpoint read-only ini tidak menutup tugas checkout, pharma atau MFA platform.
