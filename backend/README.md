# StorePOS backend

Modular monolith Go. Satu binary untuk API, worker, migrasi, dan utilitas database:

```bash
go run ./cmd/storepos api      # HTTP API /api/v1
go run ./cmd/storepos worker   # outbox, rekonsiliasi, laporan
go run ./cmd/storepos seed     # migrasi + data demo (aman dijalankan ulang)
```

Untuk pengembangan dengan reload otomatis, jalankan dari folder `backend`:

```bash
air
```

Air memakai `.air.toml` untuk membangun `./cmd/storepos` dan menjalankan API.

## Data awal pengembangan

Jalankan dari folder `backend`:

```bash
go run ./cmd/storepos seed
```

Perintah ini hanya tersedia di environment `development`/`test`. Seed mengisi tiga
jenis usaha, cabang, terminal, user semua role, kategori, produk, varian, barcode,
satuan, supplier, pelanggan, pajak, saldo stok, dan stok minimum. Semua akun memakai
password `password789`:

| Email | Role utama |
| --- | --- |
| `owner@storepos.test` | owner (semua bisnis demo) |
| `manager@storepos.test` | manager |
| `cashier@storepos.test` | cashier |
| `warehouse@storepos.test` | warehouse |
| `kitchen@storepos.test` | kitchen |
| `pharmacist@storepos.test` | pharmacist |

## Konfigurasi

Env var dibaca dari lingkungan atau berkas `.env` (opsional, tidak di-commit).

| Variabel | Default | Keterangan |
| --- | --- | --- |
| `STOREPOS_HTTP_ADDR` | `:8080` | Alamat HTTP |
| `STOREPOS_DB_URL` | — | DSN PostgreSQL |
| `STOREPOS_ENV` | `development` | `development` / `production` |
| `STOREPOS_LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error` |

## Struktur paket

```
cmd/storepos         Entry point (api/worker)
internal/platform    Fondasi lintas modul: db, httpx, money, errors, logger, authz, tenant
internal/<modul>     Modul domain (identity, org, catalog, pricing, sales, ...)
```

Modul berkomunikasi melalui interface Go. Satu transaksi database dapat melintasi modul melalui `platform.UnitOfWork`.

## Pengujian

```bash
go test ./...
```

Uji integrasi membutuhkan PostgreSQL lokal (`STOREPOS_TEST_DB_URL`).
