package org

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/storepos/storepos/internal/platform/auth"
	"github.com/storepos/storepos/internal/platform/idkit"
)

const pinMaxFailures = 5
const pinLockMinutes = 15

var (
	// ErrApprovalDenied sengaja generik: tidak membocorkan apakah email, peran, atau PIN yang salah.
	ErrApprovalDenied = errors.New("persetujuan atasan ditolak")
	ErrPINLocked      = errors.New("PIN atasan terkunci sementara")
)

// SetPIN menyimpan hash PIN 6 digit untuk keanggotaan (CORE-006).
func (r *Repo) SetPIN(ctx context.Context, membershipID idkit.ID, pin string) error {
	h, err := auth.HashPIN(pin)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`UPDATE memberships SET pin_hash=$2, pin_fail_count=0, pin_locked_until=NULL, updated_at=now() WHERE id=$1`,
		membershipID, h)
	if err != nil {
		return fmt.Errorf("org: simpan PIN: %w", err)
	}
	return nil
}

// VerifyApprover memeriksa PIN atasan (Pemilik/Manajer aktif) dan mengembalikan user ID-nya.
// Panggil dalam transaksi tersendiri dan commit penolakan PIN; FOR UPDATE menserialkan
// percobaan paralel. Jangan gabungkan dengan transaksi bisnis yang dapat rollback.
func (r *Repo) VerifyApprover(ctx context.Context, businessID, actor idkit.ID, email, pin string) (idkit.ID, error) {
	return r.VerifyRoles(ctx, businessID, actor, email, pin, "owner", "manager")
}

// VerifyRoles sama dengan VerifyApprover tetapi peran penyetuju dapat ditentukan (mis. pharmacist, APT-009).
func (r *Repo) VerifyRoles(ctx context.Context, businessID, actor idkit.ID, email, pin string, roles ...string) (idkit.ID, error) {
	var (
		mid, uid     idkit.ID
		role, status string
		hash         *string
		fails        int
		lockedUntil  *time.Time
	)
	err := r.db.QueryRow(ctx,
		`SELECT m.id, m.user_id, m.role, m.status, m.pin_hash, m.pin_fail_count, m.pin_locked_until
		 FROM memberships m JOIN users u ON u.id = m.user_id
		 WHERE m.business_id=$1 AND u.email=$2 AND u.status='active' FOR UPDATE OF m`, businessID, strings.ToLower(strings.TrimSpace(email))).
		Scan(&mid, &uid, &role, &status, &hash, &fails, &lockedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return idkit.ID{}, ErrApprovalDenied
	}
	if err != nil {
		return idkit.ID{}, fmt.Errorf("org: cari atasan: %w", err)
	}
	if status != "active" || !slices.Contains(roles, role) || hash == nil || uid == actor {
		return idkit.ID{}, ErrApprovalDenied
	}
	if lockedUntil != nil && lockedUntil.After(time.Now()) {
		return idkit.ID{}, ErrPINLocked
	}
	ok, err := auth.VerifyPIN(pin, *hash)
	if err != nil || !ok {
		_, e := r.db.Exec(ctx,
			`UPDATE memberships SET pin_fail_count = CASE WHEN pin_locked_until <= now() THEN 1 ELSE pin_fail_count + 1 END,
			   pin_locked_until = CASE WHEN (CASE WHEN pin_locked_until <= now() THEN 1 ELSE pin_fail_count + 1 END) >= $2 THEN now() + make_interval(mins => $3) END
			 WHERE id=$1`, mid, pinMaxFailures, pinLockMinutes)
		if e != nil {
			return idkit.ID{}, fmt.Errorf("org: catat PIN gagal: %w", e)
		}
		return idkit.ID{}, ErrApprovalDenied
	}
	if fails > 0 {
		if _, err := r.db.Exec(ctx, `UPDATE memberships SET pin_fail_count=0, pin_locked_until=NULL WHERE id=$1`, mid); err != nil {
			return idkit.ID{}, fmt.Errorf("org: reset PIN: %w", err)
		}
	}
	return uid, nil
}
