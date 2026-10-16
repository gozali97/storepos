# Batch 10 — pengelolaan data backend

1. GET /customers/{id}: detail/versi/anonymized, sale.create baca.
2. PUT /customers/{id}, If-Match: {name,phone,note}, sale.create tulis. Anonim tidak boleh diisi ulang. Audit versi saja, tidak PII.
3. GET /customers?q=...&after=...: pencarian nama/HP literal (wildcard escaped), anonim dikecualikan.
4. GET /suppliers/{id}: detail arsip/versi, supplier.manage.
5. GET /suppliers?q=...&after=...: nama/kontak literal, termasuk arsip seperti kontrak lama.
6. GET /invitations?after=...: Owner semua tenant, Manager invited_by sendiri. Tidak token/hash, no-store. Kontak hanya pengelola.
7. GET /branches/{id}/registers?status=active|inactive|archived|all&after=...: default active, izin branch.view dan penugasan cabang.
8. GET /settings/business-profile: {name,version}, business.manage.
9. PUT endpoint profil bisnis, If-Match: {name}, nama 1..200 karakter. Version menggunakan org_version bersama jenis usaha; audit. Nama struk lama tetap snapshot.
10. DELETE /categories/{id}, If-Match: arsip (bukan hapus), product.manage, version/status/audit. Produk lama tetap category_id; category list aktif tidak memuat arsip.

Endpoint baru tidak memerlukan migrasi. Daftar dibatasi 100 + cursor. Pencarian q maksimal 200 byte; ILIKE belum memiliki indeks trigram/performance benchmark. Detail/daftar kontak no-store pada endpoint detail baru; list pelanggan/supplier lama belum memiliki cache-control konsisten.

Frontend tidak diubah. Tidak menambah fungsi tes baru; build/vet/suite lama -race lulus, verifikasi HTTP/isolasi/konkurensi spesifik batch belum dilakukan. Catatan semua fase sebelumnya tetap terbuka di TASKS.md, ini bukan deklarasi backend lengkap.
