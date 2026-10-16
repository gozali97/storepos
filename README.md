# StorePOS

POS berbasis web multi-tenant untuk toko, kafe/restoran, dan apotek di Indonesia. Backend Go (modular monolith), frontend React, database PostgreSQL.

Dokumen utama:

- [PRD.md](./PRD.md) — kebutuhan produk v0.1
- [TASKS.md](./TASKS.md) — rencana pekerjaan per fase
- [docs/DECISIONS.md](./docs/DECISIONS.md) — asumsi kerja & nilai default
- [docs/adr/](./docs/adr/) — Architecture Decision Record

## Struktur repo

```
backend/   Go API + worker (modular monolith)
app/       Aplikasi kasir & back-office (React SPA)
landing/   Landing page statis/prerender
db/        Migrasi SQL dan seed
docs/      Dokumentasi arsitektur, keputusan, ADR
```

## Pengembangan lokal

Prasyarat: Go 1.25+, Node 20+, PostgreSQL 16, Docker (opsional untuk Compose).

Frontend terpadu: `cd app && npm run dev`, lalu buka `http://localhost:5173/` (landing), `/masuk` (login), atau `/dashboard` (admin). `npm run build` dari `app` menghasilkan landing dan aplikasi dalam `app/dist`.

```bash
# Backend
cd backend
cp .env.example .env
go run ./cmd/storepos api

# Worker (terminal lain)
go run ./cmd/storepos worker

# Database (dengan Compose)
docker compose up -d db
```

Format uang disimpan sebagai integer sen (BIGINT di DB, int64 di Go). Kuantitas disimpan sebagai NUMERIC(14,3) di DB dan milli-unit int64 di Go. Lihat §13.7 PRD.
