// Command storepos adalah entry point untuk API, worker, dan utilitas basis data.
//
//	storepos api                    # jalankan HTTP API
//	storepos worker                 # jalankan worker outbox + rekonsiliasi
//	storepos migrate up             # terapkan migrasi yang belum dijalankan
//	storepos migrate status         # tampilkan migrasi yang sudah diterapkan
//	storepos seed                   # migrasi dan isi data demo
//	storepos db ping                # uji koneksi database
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apiserver "github.com/storepos/storepos/internal/api"
	"github.com/storepos/storepos/internal/platform/config"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/logger"
	"github.com/storepos/storepos/internal/platform/migrate"
	"github.com/storepos/storepos/internal/reporting"
	"github.com/storepos/storepos/internal/seed"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel, os.Stdout).With(
		"env", string(cfg.Env),
		"cmd", os.Args[1],
	)
	slog.SetDefault(log)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var runErr error
	switch os.Args[1] {
	case "api":
		runErr = runAPI(ctx, cfg, log)
	case "worker":
		runErr = runWorker(ctx, cfg, log)
	case "migrate":
		runErr = runMigrate(ctx, cfg, log, os.Args[2:])
	case "db":
		runErr = runDB(ctx, cfg, log, os.Args[2:])
	case "seed":
		runErr = runSeed(ctx, cfg, log)
	default:
		usage()
		os.Exit(2)
	}

	if runErr != nil {
		log.Error("perintah gagal", "err", runErr)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: storepos <perintah>

perintah:
  api              jalankan HTTP API
  worker           jalankan worker latar
  migrate up       terapkan migrasi
  migrate down     balikkan migrasi terakhir
  migrate status   tampilkan status migrasi
  seed             migrasi dan isi data demo pengembangan
  db ping          uji koneksi database`)
}

// ---------------------------------------------------------------------------
// API
// ---------------------------------------------------------------------------

func runAPI(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("hubungkan database: %w", err)
	}
	defer pool.Close()

	ttl, err := time.ParseDuration(cfg.SessionAbsoluteTTL)
	if err != nil {
		return fmt.Errorf("STOREPOS_SESSION_ABSOLUTE_TTL tidak valid: %w", err)
	}
	api, err := apiserver.NewServer(pool, ttl, cfg.Env == config.EnvProduction)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	registerRoutes(mux, pool)
	api.Routes(mux)
	metrics := httpx.NewMetrics()
	mux.HandleFunc("GET /internal/metrics", metrics.Handler)

	handler := httpx.RequestIDMiddleware(log)(
		httpx.Recover(
			httpx.AccessLogMiddleware(metrics.Middleware(mux)),
		),
	)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal diterima")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func registerRoutes(mux *http.ServeMux, pool *database.Pool) {
	mux.HandleFunc("GET /healthz", livenessHandler)
	mux.HandleFunc("GET /api/v1/health", readinessHandler(pool))
}

// livenessHandler hanya menyatakan proses hidup; tidak menyentuh database.
func livenessHandler(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"service":    "storepos",
		"request_id": httpx.RequestID(r.Context()),
	})
}

// readinessHandler memeriksa dependensi (database). Dipakai load balancer.
func readinessHandler(pool *database.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		dbStatus := "ok"
		status := http.StatusOK
		if err := pool.Ping(ctx); err != nil {
			dbStatus = "error"
			status = http.StatusServiceUnavailable
			httpx.Logger(ctx).Error("health: database tidak merespons", "err", err)
		}

		httpx.JSON(w, status, map[string]any{
			"status":     map[bool]string{true: "ok", false: "degraded"}[status == http.StatusOK],
			"service":    "storepos",
			"database":   dbStatus,
			"request_id": httpx.RequestID(r.Context()),
		})
	}
}

// ---------------------------------------------------------------------------
// Worker
// ---------------------------------------------------------------------------

func runWorker(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("hubungkan database: %w", err)
	}
	defer pool.Close()

	log.Info("worker rekonsiliasi siap", "poll_interval", "5s", "schedule_timezone", "UTC")
	return reporting.Worker(ctx, pool, log)
}

// ---------------------------------------------------------------------------
// Migrasi
// ---------------------------------------------------------------------------

func runMigrate(ctx context.Context, cfg config.Config, log *slog.Logger, args []string) error {
	if len(args) == 0 {
		return errors.New("subperintah migrate wajib: up | down | status")
	}

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("hubungkan database: %w", err)
	}
	defer pool.Close()

	runner := migrate.New(pool)

	switch args[0] {
	case "up":
		ran, err := runner.Up(ctx)
		if err != nil {
			return err
		}
		if len(ran) == 0 {
			log.Info("tidak ada migrasi baru")
			return nil
		}
		for _, name := range ran {
			log.Info("migrasi diterapkan", "migrasi", name)
		}
		log.Info("migrasi selesai", "jumlah", len(ran))
		return nil
	case "down":
		name, err := runner.Down(ctx)
		if errors.Is(err, migrate.ErrNothingToRollback) {
			log.Info("tidak ada migrasi untuk dibalik")
			return nil
		}
		if err != nil {
			return err
		}
		log.Info("migrasi dibalik", "migrasi", name)
		return nil
	case "status":
		rows, err := runner.Status(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("%-8s %-32s %-22s %s\n", "VERSI", "NAMA", "TERAPAN_PADA", "STATUS")
		for _, s := range rows {
			applied := "belum"
			at := "-"
			if s.AppliedAt != nil {
				applied = "ya"
				at = s.AppliedAt.UTC().Format(time.RFC3339)
			}
			fmt.Printf("%-8d %-32s %-22s %s\n", s.Version, s.Name, at, applied)
		}
		return nil
	default:
		return fmt.Errorf("subperintah migrate tidak dikenal: %q", args[0])
	}
}

// ---------------------------------------------------------------------------
// Utilitas basis data
// ---------------------------------------------------------------------------

func runDB(ctx context.Context, cfg config.Config, log *slog.Logger, args []string) error {
	if len(args) == 0 || args[0] != "ping" {
		return errors.New("usage: storepos db ping")
	}

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	start := time.Now()
	if err := pool.Ping(ctx); err != nil {
		return err
	}
	log.Info("database ok", "latency_ms", time.Since(start).Milliseconds())
	return nil
}

func runSeed(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	if cfg.Env == config.EnvProduction {
		return errors.New("seed data demo tidak boleh dijalankan di production")
	}
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("hubungkan database: %w", err)
	}
	defer pool.Close()

	if _, err := migrate.New(pool).Up(ctx); err != nil {
		return fmt.Errorf("migrasi sebelum seed: %w", err)
	}
	result, err := seed.Apply(ctx, pool)
	if err != nil {
		return err
	}
	log.Info("seed selesai", "businesses", result.Businesses, "accounts", len(result.Accounts))
	for _, account := range result.Accounts {
		fmt.Printf("%-28s role=%s\n", account.Email, account.Role)
	}
	fmt.Printf("password semua akun: %s\n", seed.Password)
	return nil
}
