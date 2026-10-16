// Package testdb membuat database PostgreSQL sementara per uji, lengkap dengan
// migrasi, lalu menghapusnya. Uji yang memakai ini tidak mengotori database
// pengembangan, dan tetap bisa menguji tabel append-only (audit, pergerakan stok).
// Uji dilewati bila PostgreSQL tidak tersedia.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/storepos/storepos/internal/platform/config"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/migrate"
)

// New mengembalikan pool ke database baru yang sudah termigrasi.
func New(t testing.TB) *database.Pool {
	t.Helper()
	if f := findEnvFile(); f != "" {
		t.Setenv("STOREPOS_ENV_FILE", f)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("config tidak tersedia: %v", err)
	}
	u, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		t.Skipf("DSN bukan URL: %v", err)
	}

	ctx := context.Background()
	admin := *u
	admin.Path = "/postgres"
	conn, err := pgx.Connect(ctx, admin.String())
	if err != nil {
		t.Skipf("database tidak tersedia: %v", err)
	}
	defer conn.Close(ctx)

	var b [6]byte
	_, _ = rand.Read(b[:])
	name := "sp_test_" + hex.EncodeToString(b[:]) // hanya heks: aman sebagai identifier
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Skipf("tidak bisa membuat database uji: %v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(ctx, admin.String())
		if err != nil {
			return
		}
		defer c.Close(ctx)
		_, _ = c.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})

	u.Path = "/" + name
	pool, err := database.Connect(ctx, u.String())
	if err != nil {
		t.Fatalf("hubungkan database uji: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := migrate.New(pool).Up(ctx); err != nil {
		t.Fatalf("migrasi database uji: %v", err)
	}
	return pool
}

// findEnvFile naik dari direktori kerja sampai folder berisi go.mod dan
// memakai .env di sana (backend/.env).
func findEnvFile() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			p := filepath.Join(dir, ".env")
			if _, err := os.Stat(p); err == nil {
				return p
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
