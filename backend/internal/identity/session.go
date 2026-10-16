package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/storepos/storepos/internal/platform/auth"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

// SessionRepo mengelola tabel sessions.
//
// Yang disimpan hanyalah SHA-256 token; token mentah hanya ada di cookie browser.
type SessionRepo struct {
	db database.Getter
}

// NewSessionRepo membuat repository sesi.
func NewSessionRepo(db database.Getter) *SessionRepo { return &SessionRepo{db: db} }

// Create menyisipkan sesi baru dan mengembalikan token mentah.
func (r *SessionRepo) Create(ctx context.Context, userID idkit.ID, ttl time.Duration, userAgent, ip string) (string, *Session, error) {
	token, err := auth.NewSessionToken()
	if err != nil {
		return "", nil, err
	}

	s := &Session{
		ID:        idkit.New(),
		UserID:    userID,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(ttl),
		UserAgent: truncate(userAgent, 512),
		IP:        ip,
	}

	const q = `
INSERT INTO sessions (id, user_id, token_hash, expires_at, user_agent, ip)
VALUES ($1, $2, $3, $4, $5, $6)`

	var userAgentArg any = s.UserAgent
	if s.UserAgent == "" {
		userAgentArg = nil
	}
	var ipArg any = s.IP
	if s.IP == "" {
		ipArg = nil
	}

	if _, err := r.db.Exec(ctx, q, s.ID, userID, auth.HashToken(token), s.ExpiresAt, userAgentArg, ipArg); err != nil {
		return "", nil, fmt.Errorf("identity: buat sesi: %w", err)
	}

	return token, s, nil
}

// ByToken mengambil sesi aktif berdasarkan token mentah dari cookie.
//
// Pencarian memakai token_hash (indeks unik), sehingga biayanya tetap O(log n)
// tidak peduli berapa banyak sesi aktif.
func (r *SessionRepo) ByToken(ctx context.Context, rawToken string) (*Session, error) {
	const q = `
SELECT id, user_id, active_business_id, created_at, expires_at, last_seen_at,
       revoked_at, COALESCE(user_agent, ''), COALESCE(host(ip), '')
FROM sessions
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > now()`

	hash := auth.HashToken(rawToken)
	var s Session
	err := r.db.QueryRow(ctx, q, hash).Scan(
		&s.ID, &s.UserID, &s.ActiveBusinessID, &s.CreatedAt, &s.ExpiresAt,
		&s.LastSeenAt, &s.RevokedAt, &s.UserAgent, &s.IP,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSessionAbsent
	}
	if err != nil {
		return nil, fmt.Errorf("identity: baca sesi: %w", err)
	}
	return &s, nil
}

// Touch memperbarui last_seen_at. Dipanggil paling banyak sekali per menit
// per sesi agar tidak menulis berlebihan (diatur oleh pemanggil).
func (r *SessionRepo) Touch(ctx context.Context, id idkit.ID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE sessions SET last_seen_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("identity: perbarui last_seen: %w", err)
	}
	return nil
}

// SetActiveBusiness menyimpan bisnis aktif pada sesi sehingga header
// X-Business-ID tidak wajib di setiap request.
func (r *SessionRepo) SetActiveBusiness(ctx context.Context, id, businessID idkit.ID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE sessions SET active_business_id = $2 WHERE id = $1`, id, businessID)
	if err != nil {
		return fmt.Errorf("identity: set bisnis aktif: %w", err)
	}
	return nil
}

// Revoke mencabut satu sesi.
func (r *SessionRepo) Revoke(ctx context.Context, id idkit.ID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("identity: cabut sesi: %w", err)
	}
	return nil
}

// RevokeAllForUser mencabut seluruh sesi pengguna.
//
// Dipakai saat pengguna dinonaktifkan: PRD CORE-004 mewajibkan pencabutan ≤ 60 detik.
// Karena sesi diperiksa ke database pada setiap request (bukan JWT), efeknya
// seketika — tidak ada jendela tunggu.
func (r *SessionRepo) RevokeAllForUser(ctx context.Context, userID idkit.ID) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE sessions SET revoked_at = now()
		 WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	if err != nil {
		return 0, fmt.Errorf("identity: cabut semua sesi: %w", err)
	}
	return tag.RowsAffected(), nil
}

// RevokeAllForUserExcept mencabut semua sesi pengguna kecuali satu (sesi saat ini),
// dipakai setelah perubahan kata sandi.
func (r *SessionRepo) RevokeAllForUserExcept(ctx context.Context, userID, keep idkit.ID) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE sessions SET revoked_at = now()
		 WHERE user_id = $1 AND id <> $2 AND revoked_at IS NULL`, userID, keep)
	if err != nil {
		return 0, fmt.Errorf("identity: cabut sesi lain: %w", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteExpired menghapus sesi kedaluwarsa/dicabut yang lebih tua dari retensi.
// Dijalankan oleh worker, bukan pada jalur permintaan.
func (r *SessionRepo) DeleteExpired(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM sessions
		 WHERE (expires_at < now() OR revoked_at IS NOT NULL)
		   AND GREATEST(expires_at, COALESCE(revoked_at, expires_at)) < $1`,
		time.Now().UTC().Add(-olderThan))
	if err != nil {
		return 0, fmt.Errorf("identity: hapus sesi kedaluwarsa: %w", err)
	}
	return tag.RowsAffected(), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
