package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadUnknownEnv(t *testing.T) {
	t.Setenv("STOREPOS_DB_URL", "postgres://x")
	t.Setenv("STOREPOS_ENV", "staging")
	if _, err := Load(); err == nil {
		t.Fatal("expected unknown env error")
	}
}

func TestLoadProductionNeedsSessionSecret(t *testing.T) {
	t.Setenv("STOREPOS_DB_URL", "postgres://x")
	t.Setenv("STOREPOS_ENV", "production")
	t.Setenv("STOREPOS_SESSION_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when session secret missing in production")
	}

	t.Setenv("STOREPOS_SESSION_SECRET", "a-very-long-session-secret-32bytes!")
	if _, err := Load(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadDevelopmentHasFallbackDSN(t *testing.T) {
	for _, k := range []string{"STOREPOS_DB_URL", "DB_HOST", "DB_NAME", "DB_USER"} {
		os.Unsetenv(k)
		t.Setenv(k, "")
	}
	t.Setenv("STOREPOS_ENV", "development")

	c, err := Load()
	if err != nil {
		t.Fatalf("development harus punya DSN bawaan: %v", err)
	}
	if !strings.Contains(c.DatabaseURL, "storepos") {
		t.Fatalf("DSN bawaan tak terduga: %s", c.DatabaseURL)
	}
}

func TestDatabaseURLFromParts(t *testing.T) {
	t.Setenv("STOREPOS_DB_URL", "")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "toko")
	t.Setenv("DB_PASSWORD", "p@ss word/x")
	t.Setenv("DB_NAME", "store_pos")
	t.Setenv("DB_SSLMODE", "disable")

	dsn, err := DatabaseURLFromEnv(EnvDevelopment)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.HasPrefix(dsn, "postgres://toko:") {
		t.Fatalf("DSN tak terduga: %s", dsn)
	}
	// Karakter khusus di kata sandi harus ter-encode.
	if strings.Contains(dsn, "p@ss word/x") {
		t.Fatalf("kata sandi tidak di-escape: %s", dsn)
	}
	if !strings.Contains(dsn, "127.0.0.1:5433") || !strings.Contains(dsn, "/store_pos") {
		t.Fatalf("DSN tak terduga: %s", dsn)
	}
	if !strings.Contains(dsn, "sslmode=disable") {
		t.Fatalf("sslmode hilang: %s", dsn)
	}
}

func TestDatabaseURLProductionRequiresExplicit(t *testing.T) {
	t.Setenv("STOREPOS_DB_URL", "")
	for _, k := range []string{"DB_HOST", "DB_NAME", "DB_USER"} {
		t.Setenv(k, "")
	}
	if _, err := DatabaseURLFromEnv(EnvProduction); err == nil {
		t.Fatal("production harus menolak tanpa DSN eksplisit")
	}
}

func TestDatabaseURLRejectsBadPort(t *testing.T) {
	t.Setenv("STOREPOS_DB_URL", "")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_NAME", "db")
	t.Setenv("DB_USER", "u")
	t.Setenv("DB_PORT", "abc")
	if _, err := DatabaseURLFromEnv(EnvDevelopment); err == nil {
		t.Fatal("port non-numerik harus error")
	}
}

func TestLoadDotEnvDoesNotOverrideExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# komentar\nSTOREPOS_HTTP_ADDR=:9999\nDB_NAME=\"dari_file\"\nexport DB_USER=dari_file_user\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STOREPOS_ENV_FILE", path)
	t.Setenv("STOREPOS_HTTP_ADDR", ":1234") // sudah ada → tidak ditimpa
	// Kosongkan agar berkas .env boleh mengisinya (nilai kosong tetap "ada").
	os.Unsetenv("DB_NAME")
	os.Unsetenv("DB_USER")
	t.Cleanup(func() {
		os.Unsetenv("DB_NAME")
		os.Unsetenv("DB_USER")
	})

	if err := loadDotEnv(); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}
	if got := os.Getenv("STOREPOS_HTTP_ADDR"); got != ":1234" {
		t.Fatalf("environment nyata harus menang, dapat %q", got)
	}
	if got := os.Getenv("DB_NAME"); got != "dari_file" {
		t.Fatalf("kutip ganda harus dilepas, dapat %q", got)
	}
	if got := os.Getenv("DB_USER"); got != "dari_file_user" {
		t.Fatalf("prefix export harus dilepas, dapat %q", got)
	}
}

func TestLoadDotEnvMissingFileIsFine(t *testing.T) {
	t.Setenv("STOREPOS_ENV_FILE", filepath.Join(t.TempDir(), "tidak-ada.env"))
	if err := loadDotEnv(); err != nil {
		t.Fatalf("berkas hilang bukan error: %v", err)
	}
}

func TestLoadDotEnvRejectsMalformedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("BARIS_TANPA_SAMA_DENGAN\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STOREPOS_ENV_FILE", path)
	if err := loadDotEnv(); err == nil {
		t.Fatal("baris rusak harus error")
	}
}
