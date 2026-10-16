// Package database menyediakan koneksi PostgreSQL dan unit of work.
//
// Satu operasi bisnis = satu transaksi database (PRD §13.12). Semua modul
// menerima UnitOfWork yang membawa transaksi aktif, sehingga finalisasi penjualan
// dapat mengubah stok, pembayaran, shift, dan audit dalam satu commit.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool membungkus pgxpool dengan konvensi aplikasi.
type Pool struct {
	pool *pgxpool.Pool
}

// Connect membuka koneksi pool dengan pengaturan konservatif.
func Connect(ctx context.Context, dsn string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	// Set timezone UTC untuk semua sesi; tanggal bisnis dihitung di aplikasi
	// memakai zona cabang (PRD §13.10).
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Pool{pool: pool}, nil
}

// Close menutup pool.
func (p *Pool) Close() {
	if p != nil && p.pool != nil {
		p.pool.Close()
	}
}

// Ping menguji koneksi.
func (p *Pool) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

// Underlying mengembalikan pgxpool untuk kasus lanjutan (jarang dipakai).
func (p *Pool) Underlying() *pgxpool.Pool { return p.pool }

// Getter adalah abstraksi minimal yang dipenuhi *pgxpool.Pool dan pgx.Tx.
// Repository menerima Getter agar dapat dipakai baik di dalam maupun luar transaksi.
type Getter interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Tx adalah interface transaksi agar repository tidak bergantung ke pgx langsung.
type Tx interface {
	Getter
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// InTx menjalankan fn dalam satu transaksi database.
//
// Optimasi retry (PRD §13.5, E24): serialization failure / deadlock di-retry
// maksimal maxRetry kali dengan backoff. Retry aman karena seluruh operasi
// berada dalam satu transaksi.
func (p *Pool) InTx(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error {
	const maxRetry = 3
	var lastErr error
	for attempt := 0; attempt <= maxRetry; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 20 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin: %w", err)
		}

		err = fn(ctx, tx)
		if err == nil {
			if err := tx.Commit(ctx); err != nil {
				if isRetryable(err) {
					lastErr = err
					continue
				}
				return fmt.Errorf("commit: %w", err)
			}
			return nil
		}

		// Rollback; error rollback diabaikan karena error asli lebih penting.
		_ = tx.Rollback(ctx)

		if isRetryable(err) {
			lastErr = err
			continue
		}
		return err
	}
	return fmt.Errorf("transaksi gagal setelah %d percobaan: %w", maxRetry+1, lastErr)
}

// isRetryable mengenali deadlock (40P01) dan serialization failure (40001).
func isRetryable(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "40P01", "40001":
			return true
		}
	}
	return false
}
