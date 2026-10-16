# ADR 0003 — Autentikasi dengan Sesi Server-Side

- Status: Diterima
- Sumber: PRD §16.5, CORE-004

## Keputusan

Sesi disimpan di PostgreSQL (tabel `sessions`). Klien memakai cookie `HttpOnly`, `Secure`, `SameSite=Lax`. Request yang mengubah data memakai token CSRF (double-submit cookie).

Alasan: pencabutan sesi instan dalam ≤ 60 detik lebih sederhana dibanding JWT tanpa infrastruktur blacklist tambahan.

## Rotasi & kadaluwarsa

- Umur sesi 24 jam idle, 7 hari absolut (default; dapat diubah per bisnis fase berikutnya).
- Logout menandai `revoked_at` pada baris sesi.
- Menonaktifkan pengguna membatalkan semua sesinya ≤ 60 detik (CORE-004).

## Multi-bisnis

Satu sesi terkait satu user global. Bisnis aktif dikirim per request lewat header `X-Business-ID` dan divalidasi middleware terhadap keanggotaan di DB.
