// Package identity mengelola pengguna global, kredensial, dan sesi.
//
// Cakupan (PRD §7.1, CORE-001, CORE-004):
//   - registrasi akun global (email/nomor telepon)
//   - login/logout dengan sesi server-side
//   - pembacaan profil pengguna dan daftar keanggotaan bisnis
//
// Modul ini TIDAK mengurus peran/izin per bisnis — itu milik modul org.
package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

// Error tingkat domain modul identity.
var (
	ErrEmailTaken     = errors.New("email sudah terdaftar")
	ErrPhoneTaken     = errors.New("nomor telepon sudah terdaftar")
	ErrUserNotFound   = errors.New("pengguna tidak ditemukan")
	ErrWrongPassword  = errors.New("kata sandi salah")
	ErrUserDisabled   = errors.New("akun dinonaktifkan")
	ErrSessionAbsent  = errors.New("sesi tidak ditemukan")
	ErrSessionExpired = errors.New("sesi kedaluwarsa")
)

// UserStatus mengikuti kolom users.status pada migrasi baseline.
type UserStatus string

const (
	UserActive   UserStatus = "active"
	UserDisabled UserStatus = "disabled"
)

// User adalah akun global. Satu pengguna dapat menjadi anggota banyak bisnis.
type User struct {
	ID              idkit.ID
	Email           string
	Phone           string
	PasswordHash    string
	Status          UserStatus
	EmailVerifiedAt *time.Time
	PhoneVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Session adalah sesi server-side (cookie hanya membawa token mentah).
type Session struct {
	ID               idkit.ID
	UserID           idkit.ID
	ActiveBusinessID *idkit.ID
	CreatedAt        time.Time
	ExpiresAt        time.Time
	LastSeenAt       time.Time
	RevokedAt        *time.Time
	UserAgent        string
	IP               string
}

// UserRepo membaca/menulis tabel users.
type UserRepo struct {
	db database.Getter
}

// NewUserRepo membuat repository pengguna.
func NewUserRepo(db database.Getter) *UserRepo { return &UserRepo{db: db} }

// Normalisasi email: huruf kecil dan tanpa spasi. Kolom email bertipe CITEXT,
// tetapi normalisasi di aplikasi membuat perilaku konsisten di seluruh lapisan.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NormalizePhone menghapus pemisah umum dan menyisakan digit dengan awalan '+'.
func NormalizePhone(phone string) string {
	p := strings.TrimSpace(phone)
	plus := strings.HasPrefix(p, "+")
	var b strings.Builder
	for _, r := range p {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	if plus {
		return "+" + b.String()
	}
	// Konvensi Indonesia: 08xx → +628xx
	d := b.String()
	if strings.HasPrefix(d, "0") {
		return "+62" + strings.TrimPrefix(d, "0")
	}
	if strings.HasPrefix(d, "62") {
		return "+" + d
	}
	return d
}

// Create menyisipkan pengguna baru.
func (r *UserRepo) Create(ctx context.Context, u *User) error {
	const q = `
INSERT INTO users (id, email, phone, password_hash, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING created_at, updated_at`

	if u.ID.IsZero() {
		u.ID = idkit.New()
	}
	email := nullIfEmpty(NormalizeEmail(u.Email))
	phone := nullIfEmpty(NormalizePhone(u.Phone))
	if email == nil && phone == nil {
		return fmt.Errorf("identity: email atau nomor telepon wajib diisi")
	}

	err := r.db.QueryRow(ctx, q, u.ID, email, phone, u.PasswordHash, string(u.Status)).
		Scan(&u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return ErrEmailTaken
		}
		if isUniqueViolation(err, "users_phone_key") {
			return ErrPhoneTaken
		}
		return fmt.Errorf("identity: buat pengguna: %w", err)
	}
	return nil
}

// ByID mengambil pengguna berdasarkan ID.
func (r *UserRepo) ByID(ctx context.Context, id idkit.ID) (*User, error) {
	const q = `
SELECT id, COALESCE(email, ''), COALESCE(phone, ''), password_hash, status,
       email_verified_at, phone_verified_at, created_at, updated_at
FROM users WHERE id = $1`
	return r.scanOne(ctx, q, id)
}

// ByEmail mengambil pengguna berdasarkan email (tanpa memperhatikan besar/kecil huruf).
func (r *UserRepo) ByEmail(ctx context.Context, email string) (*User, error) {
	const q = `
SELECT id, COALESCE(email, ''), COALESCE(phone, ''), password_hash, status,
       email_verified_at, phone_verified_at, created_at, updated_at
FROM users WHERE email = $1`
	return r.scanOne(ctx, q, NormalizeEmail(email))
}

// ByPhone mengambil pengguna berdasarkan nomor telepon yang sudah dinormalisasi.
func (r *UserRepo) ByPhone(ctx context.Context, phone string) (*User, error) {
	const q = `
SELECT id, COALESCE(email, ''), COALESCE(phone, ''), password_hash, status,
       email_verified_at, phone_verified_at, created_at, updated_at
FROM users WHERE phone = $1`
	return r.scanOne(ctx, q, NormalizePhone(phone))
}

// ByLogin mencari pengguna berdasarkan email ATAU nomor telepon.
func (r *UserRepo) ByLogin(ctx context.Context, identifier string) (*User, error) {
	id := strings.TrimSpace(identifier)
	if strings.Contains(id, "@") {
		return r.ByEmail(ctx, id)
	}
	if u, err := r.ByPhone(ctx, id); err == nil {
		return u, nil
	} else if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}
	// Bisa jadi pengguna mengetik email tanpa '@' (tidak lazim) — coba sebagai email.
	return r.ByEmail(ctx, id)
}

func (r *UserRepo) scanOne(ctx context.Context, q string, args ...any) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx, q, args...).Scan(
		&u.ID, &u.Email, &u.Phone, &u.PasswordHash, &u.Status,
		&u.EmailVerifiedAt, &u.PhoneVerifiedAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("identity: baca pengguna: %w", err)
	}
	return &u, nil
}

// UpdatePasswordHash mengganti hash kata sandi (dipakai saat reset dan rehash).
func (r *UserRepo) UpdatePasswordHash(ctx context.Context, id idkit.ID, hash string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, id, hash)
	if err != nil {
		return fmt.Errorf("identity: perbarui kata sandi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// SetStatus mengaktifkan/menonaktifkan pengguna.
func (r *UserRepo) SetStatus(ctx context.Context, id idkit.ID, s UserStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET status = $2, updated_at = now() WHERE id = $1`, id, string(s))
	if err != nil {
		return fmt.Errorf("identity: ubah status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// MarkEmailVerified menandai email terverifikasi.
func (r *UserRepo) MarkEmailVerified(ctx context.Context, id idkit.ID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET email_verified_at = now(), updated_at = now()
		 WHERE id = $1 AND email_verified_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("identity: verifikasi email: %w", err)
	}
	_ = tag
	return nil
}
