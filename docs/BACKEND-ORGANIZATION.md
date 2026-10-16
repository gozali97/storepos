# Cabang dan terminal — CORE-003

Owner dengan branch.manage:

- POST /branches: `{code,name,timezone,address,receipt_header,receipt_footer}`. Zona IANA wajib valid, default Asia/Jakarta; kode uppercase/digit/hyphen maksimal 20 byte. Respons branch_id/version.
- PUT /branches/{id}, If-Match: ganti konfigurasi. Kode tidak dapat diedit melalui endpoint ini. Zona waktu terkunci setelah penjualan pertama atau saat shift masih terbuka agar tanggal bisnis historis konsisten.
- POST /branches/{id}/registers: `{name}` membuat terminal aktif.
- PATCH /registers/{id}/status, If-Match: active/inactive. Inactive ditolak bila shift terbuka.

Pembuatan/perubahan teraudit. Versi stale ditolak. Buka shift mengunci cabang/terminal dan memeriksa status agar tidak berlomba dengan penonaktifan terminal. Pembacaan cabang/terminal melalui workspace yang sudah tersedia; Manager tidak diberi izin mutasi.

Migrasi 0025 menambah version cabang/terminal. Belum: arsip cabang, edit nama terminal, pajak/biaya per cabang, batas paket, serta UI. GET /branches/{id} dan GET /registers/{id} kini mengembalikan konfigurasi/versi dengan izin branch.view dan akses cabang. Frontend tidak diubah.

Validasi terbaru: build/vet dan seluruh suite backend -race lulus. Bug ID aturan pajak seed yang sama untuk tiga tenant diperbaiki; setiap tenant kini memiliki ID berbeda, dengan uji satu pajak aktif per tenant dan seed berulang. Database development tidak di-seed ulang otomatis untuk menghindari menimpa data operasional.
