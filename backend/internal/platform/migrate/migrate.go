// Package migrate menjalankan migrasi SQL dengan aman dan dapat diulang.
//
// Pendekatan: setiap berkas embedded di internal/platform/migrate/sql bernama
// NNNN_nama.up.sql / NNNN_nama.down.sql. Migrasi yang sudah dijalankan dicatat
// di tabel schema_migrations, dan setiap migrasi berjalan dalam transaksinya
// sendiri sehingga kegagalan parsial tidak mungkin (PRD §16.6).
package migrate

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/storepos/storepos/internal/platform/database"
)

//go:embed all:sql
var bundled embed.FS

// migrationFile adalah satu migrasi yang sudah diparse.
type migrationFile struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
}

var namePattern = regexp.MustCompile(`^(\d{4})_(.+)\.(up|down)\.sql$`)

// Runner menjalankan migrasi terhadap sebuah pool.
type Runner struct {
	pool *database.Pool
	fsys fs.FS
}

// New membuat runner yang membaca migrasi dari sistem berkas embed.
func New(pool *database.Pool) *Runner {
	sub, err := fs.Sub(bundled, "sql")
	if err != nil {
		// Tidak mungkin terjadi karena embed statis; panik lebih baik daripada
		// diam-diam menjalankan nol migrasi.
		panic(fmt.Sprintf("migrate: sub fs: %v", err))
	}
	return &Runner{pool: pool, fsys: sub}
}

// NewWithFS membuat runner dengan sumber migrasi kustom (dipakai pengujian).
func NewWithFS(pool *database.Pool, fsys fs.FS) *Runner {
	return &Runner{pool: pool, fsys: fsys}
}

// Files mengembalikan daftar migrasi yang dikenali, terurut naik.
func (r *Runner) Files() ([]string, error) {
	files, err := r.load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for _, m := range files {
		names = append(names, fmt.Sprintf("%04d_%s", m.Version, m.Name))
	}
	return names, nil
}

// Up menjalankan semua migrasi yang belum diterapkan, berurutan.
func (r *Runner) Up(ctx context.Context) ([]string, error) {
	if err := r.ensureTable(ctx); err != nil {
		return nil, err
	}

	applied, err := r.appliedVersions(ctx)
	if err != nil {
		return nil, err
	}

	files, err := r.load()
	if err != nil {
		return nil, err
	}

	var ran []string
	for _, m := range files {
		if applied[m.Version] {
			continue
		}
		if err := r.applyOne(ctx, m); err != nil {
			return ran, fmt.Errorf("migrasi %04d_%s: %w", m.Version, m.Name, err)
		}
		ran = append(ran, fmt.Sprintf("%04d_%s", m.Version, m.Name))
	}
	return ran, nil
}

// Down membalik migrasi terakhir yang sudah diterapkan.
// Hanya satu langkah per pemanggilan agar operator dapat berhenti di antaranya.
func (r *Runner) Down(ctx context.Context) (string, error) {
	if err := r.ensureTable(ctx); err != nil {
		return "", err
	}

	applied, err := r.appliedVersions(ctx)
	if err != nil {
		return "", err
	}
	if len(applied) == 0 {
		return "", ErrNothingToRollback
	}

	files, err := r.load()
	if err != nil {
		return "", err
	}

	// Cari versi terbesar yang sudah diterapkan.
	target := -1
	for _, m := range files {
		if applied[m.Version] && m.Version > target {
			target = m.Version
		}
	}
	if target < 0 {
		return "", ErrNothingToRollback
	}

	var chosen migrationFile
	for _, m := range files {
		if m.Version == target {
			chosen = m
			break
		}
	}
	if chosen.DownSQL == "" {
		return "", fmt.Errorf("migrasi %04d_%s tidak punya berkas .down.sql", chosen.Version, chosen.Name)
	}

	if err := r.rollbackOne(ctx, chosen); err != nil {
		return "", fmt.Errorf("rollback %04d_%s: %w", chosen.Version, chosen.Name, err)
	}
	return fmt.Sprintf("%04d_%s", chosen.Version, chosen.Name), nil
}

// ErrNothingToRollback dikembalikan bila tidak ada migrasi untuk dibalik.
var ErrNothingToRollback = errors.New("tidak ada migrasi untuk dibalik")

// rollbackOne menjalankan skrip down dan menghapus catatannya dalam satu transaksi.
func (r *Runner) rollbackOne(ctx context.Context, m migrationFile) error {
	sql := stripTxnWrapper(m.DownSQL)

	conn, err := r.pool.Underlying().Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire: %w", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM schema_migrations WHERE version = $1`, m.Version); err != nil {
		return fmt.Errorf("hapus catatan migrasi: %w", err)
	}
	return tx.Commit(ctx)
}

// Status melaporkan setiap migrasi yang dikenal beserta waktu penerapannya.
func (r *Runner) Status(ctx context.Context) ([]Status, error) {
	if err := r.ensureTable(ctx); err != nil {
		return nil, err
	}

	applied := map[int]time.Time{}
	rows, err := r.pool.Underlying().Query(ctx,
		`SELECT version, applied_at FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("baca schema_migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v int
		var at time.Time
		if err := rows.Scan(&v, &at); err != nil {
			return nil, err
		}
		applied[v] = at
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	files, err := r.load()
	if err != nil {
		return nil, err
	}

	out := make([]Status, 0, len(files))
	for _, m := range files {
		s := Status{Version: m.Version, Name: m.Name}
		if at, ok := applied[m.Version]; ok {
			appliedAt := at
			s.AppliedAt = &appliedAt
			s.Applied = true
		}
		out = append(out, s)
	}
	return out, nil
}

// Status menggambarkan satu migrasi.
type Status struct {
	Version   int
	Name      string
	Applied   bool
	AppliedAt *time.Time
}

// appliedVersions membaca versi yang sudah diterapkan.
func (r *Runner) appliedVersions(ctx context.Context) (map[int]bool, error) {
	rows, err := r.pool.Underlying().Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("baca schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func (r *Runner) ensureTable(ctx context.Context) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     INT PRIMARY KEY,
    name        TEXT NOT NULL,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);`
	if _, err := r.pool.Underlying().Exec(ctx, ddl); err != nil {
		return fmt.Errorf("buat schema_migrations: %w", err)
	}
	return nil
}

// applyOne menjalankan satu migrasi beserta pencatatannya dalam satu transaksi.
//
// Catatan penting: file migrasi pada repo ini sudah memuat BEGIN/COMMIT sendiri.
// Untuk menghindari transaksi bersarang, pembungkus BEGIN/COMMIT di luar dihapus
// dan pgx menjalankan seluruh skrip sebagai satu implicit transaction batch.
func (r *Runner) applyOne(ctx context.Context, m migrationFile) error {
	sql := stripTxnWrapper(m.UpSQL)

	conn, err := r.pool.Underlying().Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire: %w", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		m.Version, m.Name); err != nil {
		return fmt.Errorf("catat migrasi: %w", err)
	}
	return tx.Commit(ctx)
}

// load membaca dan memvalidasi seluruh berkas migrasi.
func (r *Runner) load() ([]migrationFile, error) {
	entries, err := fs.ReadDir(r.fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("baca direktori migrasi: %w", err)
	}

	byVersion := map[int]*migrationFile{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		matches := namePattern.FindStringSubmatch(e.Name())
		if matches == nil {
			return nil, fmt.Errorf("nama berkas migrasi tidak valid: %s (harus NNNN_nama.up.sql)", e.Name())
		}
		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("versi tidak valid pada %s: %w", e.Name(), err)
		}
		name := matches[2]
		direction := matches[3]

		body, err := fs.ReadFile(r.fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("baca %s: %w", e.Name(), err)
		}

		entry, ok := byVersion[version]
		if !ok {
			entry = &migrationFile{Version: version, Name: name}
			byVersion[version] = entry
		}
		if entry.Name != name {
			return nil, fmt.Errorf("versi %04d dipakai oleh dua nama berbeda: %s dan %s", version, entry.Name, name)
		}
		switch direction {
		case "up":
			if entry.UpSQL != "" {
				return nil, fmt.Errorf("berkas up ganda untuk versi %04d", version)
			}
			entry.UpSQL = string(body)
		case "down":
			if entry.DownSQL != "" {
				return nil, fmt.Errorf("berkas down ganda untuk versi %04d", version)
			}
			entry.DownSQL = string(body)
		}
	}

	out := make([]migrationFile, 0, len(byVersion))
	for _, m := range byVersion {
		if m.UpSQL == "" {
			return nil, fmt.Errorf("versi %04d tidak punya berkas .up.sql", m.Version)
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// stripTxnWrapper menghapus BEGIN;/COMMIT; terluar dari skrip migrasi supaya
// pgx dapat mengelolanya sendiri. BEGIN/COMMIT di dalam blok DO tidak tersentuh
// karena hanya pencocokan baris pertama/terakhir yang dilakukan.
func stripTxnWrapper(sql string) string {
	lines := strings.Split(sql, "\n")

	first := -1
	for i, l := range lines {
		s := strings.TrimSpace(l)
		if s == "" || strings.HasPrefix(s, "--") {
			continue
		}
		first = i
		break
	}
	last := -1
	for i := len(lines) - 1; i >= 0; i-- {
		s := strings.TrimSpace(lines[i])
		if s == "" || strings.HasPrefix(s, "--") {
			continue
		}
		last = i
		break
	}

	if first >= 0 && strings.EqualFold(strings.TrimSpace(lines[first]), "BEGIN;") {
		lines[first] = "-- BEGIN dihapus: transaksi dikelola runner migrasi"
	}
	if last >= 0 && strings.EqualFold(strings.TrimSpace(lines[last]), "COMMIT;") {
		lines[last] = "-- COMMIT dihapus: transaksi dikelola runner migrasi"
	}
	return strings.Join(lines, "\n")
}
