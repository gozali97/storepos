# Keanggotaan/peran — CORE-004/005

GET /api/v1/memberships?after=UUID: paginasi 100 anggota. Owner melihat semua; Manager hanya anggota operasional yang tidak ditugaskan di luar cabangnya. ID pengguna/peran/status/versi/cabang saja; tidak mengirim hash password/PIN.

PUT /api/v1/memberships/{id}, If-Match: `{role,status,branch_ids}` mengganti peran/status/penugasan seluruhnya. Role owner/manager/cashier/warehouse/kitchen/pharmacist, status active/disabled. Cabang harus tenant yang sama dan aktif, tanpa duplikat. Manager tidak dapat mengelola atau mengangkat Owner/Manager dan tidak bisa menugaskan di luar cabangnya. Pemeriksaan actor/cabang diulang dalam transaksi setelah tenant row lock supaya tidak memakai izin usang.

Owner aktif terakhir tidak dapat diturunkan/dinonaktifkan. Tenant lock menserialkan demosi bersamaan. Versi stale ditolak 409. Perubahan diaudit beserta daftar cabang/peran/status sebelum/sesudah, tanpa email atau hash PIN. Perubahan role/disable menghapus PIN lama agar tidak terbawa ke peran baru.

Disable keanggotaan mencabut sesi active_business_id yang menunjuk bisnis ini; sesi multi-bisnis tanpa active_business_id tetap ditolak oleh RequireTenant pada request tenant berikutnya, tanpa menghilangkan akses bisnis lain. /me dapat tetap bekerja untuk pemilih bisnis. Re-enable dapat memakai sesi global yang belum dicabut; kebijakan pemaksaan login ulang untuk seluruh sesi perlu diputuskan.

Migrasi 0023 menambahkan version dan FK komposit membership_branches→memberships yang mencegah penugasan lintas tenant dari DB. Migrasi sengaja gagal bila data lama sudah melanggar constraint.

Belum: undangan/acceptance, penghapusan anggota, global disable users, pencabutan sesi bisnis granular dengan token terpisah, UI. Frontend ditangani agent lain dan tidak diubah.
