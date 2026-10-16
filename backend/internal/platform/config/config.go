// Package config memuat konfigurasi aplikasi dari environment variable.
// Mengikuti pola 12-factor; nilai sensitif tidak boleh default di kode.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Env string

const (
	EnvDevelopment Env = "development"
	EnvProduction  Env = "production"
	EnvTest        Env = "test"
)

// Config adalah konfigurasi runtime yang sudah divalidasi.
type Config struct {
	HTTPAddr      string
	DatabaseURL   string
	Env           Env
	LogLevel      string
	SessionSecret string

	// Kebijakan lokal yang dapat diubah operator tanpa ubah kode (§23).
	SessionIdleTTL     string // durasi idle sesi, default "24h"
	SessionAbsoluteTTL string // durasi absolut sesi, default "168h" (7 hari)
	InvitationTTL      string // masa berlaku undangan, default "168h"
}

// Load membaca konfigurasi. Error jika variabel wajib tidak diset di production.
//
// Urutan: nilai bawaan → berkas .env (hanya development) → environment nyata.
// Environment nyata selalu menang, sehingga kontainer produksi tidak perlu .env.
func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	c := Config{
		HTTPAddr:      getenv("STOREPOS_HTTP_ADDR", ":8080"),
		Env:           Env(getenv("STOREPOS_ENV", string(EnvDevelopment))),
		LogLevel:      getenv("STOREPOS_LOG_LEVEL", "info"),
		SessionSecret: os.Getenv("STOREPOS_SESSION_SECRET"),

		SessionIdleTTL:     getenv("STOREPOS_SESSION_IDLE_TTL", "24h"),
		SessionAbsoluteTTL: getenv("STOREPOS_SESSION_ABSOLUTE_TTL", "168h"),
		InvitationTTL:      getenv("STOREPOS_INVITATION_TTL", "168h"),
	}

	switch c.Env {
	case EnvDevelopment, EnvProduction, EnvTest:
	default:
		return c, fmt.Errorf("STOREPOS_ENV tidak dikenal: %q", c.Env)
	}

	dsn, err := DatabaseURLFromEnv(c.Env)
	if err != nil {
		return c, err
	}
	c.DatabaseURL = dsn

	if c.Env == EnvProduction {
		if len(c.SessionSecret) < 32 {
			return c, errors.New("STOREPOS_SESSION_SECRET wajib ≥ 32 karakter di production")
		}
	}
	return c, nil
}

// DatabaseURLFromEnv menyusun DSN PostgreSQL.
//
// Dua bentuk didukung:
//   - STOREPOS_DB_URL (DSN lengkap, diprioritaskan)
//   - DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME/DB_SSLMODE (potongan)
func DatabaseURLFromEnv(env Env) (string, error) {
	if v := strings.TrimSpace(os.Getenv("STOREPOS_DB_URL")); v != "" {
		return v, nil
	}

	host := getenv("DB_HOST", "")
	name := getenv("DB_NAME", "")
	user := getenv("DB_USER", "")

	// Di development dan test, izinkan DSN default ke PostgreSQL lokal.
	if host == "" || name == "" || user == "" {
		if env == EnvDevelopment || env == EnvTest {
			return "postgres://localhost:5432/storepos?sslmode=disable", nil
		}
		return "", errors.New("STOREPOS_DB_URL atau DB_HOST/DB_NAME/DB_USER wajib diisi")
	}

	port := getenv("DB_PORT", "5432")
	ssl := getenv("DB_SSLMODE", "prefer")
	pass := os.Getenv("DB_PASSWORD")

	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, pass),
		Host:   host + ":" + port,
		Path:   "/" + name,
	}
	q := url.Values{}
	q.Set("sslmode", ssl)
	// Kesalahan koneksi dikembalikan ke pool; batasi ukuran pesan error.
	q.Set("application_name", "storepos")
	u.RawQuery = q.Encode()

	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("DB_PORT bukan angka: %q", port)
	}
	return u.String(), nil
}

func getenv(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

// loadDotEnv memuat berkas .env bila ada. Berkas ini opsional dan hanya untuk
// kenyamanan pengembangan lokal. Format: KEY=VALUE, komentar diawali '#'.
// Nilai yang sudah ada di environment tidak ditimpa.
func loadDotEnv() error {
	path := strings.TrimSpace(os.Getenv("STOREPOS_ENV_FILE"))
	if path == "" {
		path = ".env"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("baca %s: %w", path, err)
	}

	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("%s:%d: baris tanpa '='", path, i+1)
		}
		key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("set %s: %w", key, err)
			}
		}
	}
	return nil
}
