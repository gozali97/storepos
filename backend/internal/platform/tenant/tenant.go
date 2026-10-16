// Package tenant membawa konteks tenant/user/membership melalui request.
// Semua repository WAJIB membaca konteks ini sebelum query (PRD §13.13).
package tenant

import (
	"context"
	"errors"

	"github.com/storepos/storepos/internal/platform/idkit"
)

type ctxKey int

const ctxContext ctxKey = iota

// Role mencerminkan daftar peran di PRD §7.2.
type Role string

const (
	RoleOwner         Role = "owner"
	RoleManager       Role = "manager"
	RoleCashier       Role = "cashier"
	RoleWarehouse     Role = "warehouse"
	RoleKitchen       Role = "kitchen"
	RolePharmacist    Role = "pharmacist"
	RolePlatformAdmin Role = "platform_admin"
)

// Context memuat identitas yang dipakai middleware & repository.
type Context struct {
	UserID         idkit.ID
	BusinessID     idkit.ID // tenant_id
	MembershipID   idkit.ID
	Role           Role
	BranchIDs      []idkit.ID // cabang yang ditugaskan; kosong = tidak ditugaskan ke cabang mana pun
	EnabledModules []string   // modul aktif bisnis (CORE-002)
}

// HasBranch mengembalikan true bila branchID termasuk cabang yang ditugaskan,
// atau selalu true untuk peran Owner (akses semua cabang).
func (c Context) HasBranch(branchID idkit.ID) bool {
	if c.Role == RoleOwner {
		return true
	}
	for _, b := range c.BranchIDs {
		if b == branchID {
			return true
		}
	}
	return false
}

// WithContext menyimpan Context di context.Context request.
func WithContext(ctx context.Context, tc Context) context.Context {
	return context.WithValue(ctx, ctxContext, tc)
}

// FromContext mengambil Context yang diset middleware. Error bila tidak ada.
func FromContext(ctx context.Context) (Context, error) {
	v, ok := ctx.Value(ctxContext).(Context)
	if !ok {
		return Context{}, errors.New("tenant context tidak ditemukan")
	}
	if v.BusinessID.IsZero() {
		return Context{}, errors.New("tenant context tidak lengkap: business_id kosong")
	}
	return v, nil
}

// MustFromContext mengembalikan Context atau panic (hanya untuk kode yang pasti dilindungi middleware).
func MustFromContext(ctx context.Context) Context {
	tc, err := FromContext(ctx)
	if err != nil {
		panic(err)
	}
	return tc
}
