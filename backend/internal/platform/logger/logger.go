// Package logger menyediakan slog.Logger yang terkonfigurasi untuk aplikasi.
// Field data pribadi dilarang (NFR-019); gunakan ID stabil, bukan nama/telepon.
package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// New mengembalikan logger JSON terstruktur.
func New(level string, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}
	lvl := parseLevel(level)
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:     lvl,
		AddSource: false,
	})
	return slog.New(h)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
