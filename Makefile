.PHONY: help backend-test backend-test-race backend-vet backend-build backend-run \
        db-create migrate-up migrate-down migrate-status db-ping pricing-vectors app-test app-build \
        landing-dev landing-build landing-test check

MIGDIR := backend/internal/platform/migrate/sql

help: ## Daftar perintah
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# --- Backend -----------------------------------------------------------------

backend-test: ## Jalankan uji unit backend
	cd backend && go test ./...

backend-test-race: ## Uji dengan race detector
	cd backend && go test -race ./...

backend-vet: ## go vet
	cd backend && go vet ./...

backend-build: ## Build binary storepos
	cd backend && go build -o bin/storepos ./cmd/storepos

backend-run: ## Jalankan API lokal (baca backend/.env)
	cd backend && go run ./cmd/storepos api

# --- Basis data --------------------------------------------------------------

db-create: ## Buat database store_pos bila belum ada (psql)
	@psql "$${DATABASE_URL:-postgres://root@127.0.0.1:5432/postgres?sslmode=disable}" \
		-tAc "SELECT 1 FROM pg_database WHERE datname='store_pos'" | grep -q 1 \
		|| psql "$${DATABASE_URL:-postgres://root@127.0.0.1:5432/postgres?sslmode=disable}" \
		-c "CREATE DATABASE store_pos"

migrate-up: ## Terapkan migrasi lewat runner Go (butuh DB_* di backend/.env)
	cd backend && go run ./cmd/storepos migrate up

migrate-down: ## Balikkan migrasi terakhir
	cd backend && go run ./cmd/storepos migrate down

migrate-status: ## Tampilkan status migrasi
	cd backend && go run ./cmd/storepos migrate status

db-ping: ## Uji koneksi database
	cd backend && go run ./cmd/storepos db ping

migrations-list: ## Tampilkan berkas migrasi yang dikenali runner
	@ls -1 $(MIGDIR)

# --- Frontend ----------------------------------------------------------------

app-install: ## Pasang dependensi frontend
	cd app && npm install

app-test: ## Uji unit frontend (Node test runner)
	cd app && npm test

app-build: ## Build produksi frontend
	cd app && npm run build

landing-dev: ## Jalankan landing StorePOS di port 5174
	cd landing && npm run dev

landing-build: ## Build landing statis terpisah dari aplikasi
	cd landing && npm run build

landing-test: ## Uji form dan payload analytics landing
	cd landing && npm test

# --- Gabungan ----------------------------------------------------------------

pricing-vectors: ## Uji vektor pricing di backend DAN frontend
	cd backend && go test -v -run TestSharedVectors ./internal/pricing/
	cd app && node --test src/pricing/engine.test.js

check: backend-vet backend-test app-test pricing-vectors ## Semua pemeriksaan cepat
