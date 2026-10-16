// Package org mengelola tenant (bisnis), cabang, terminal, keanggotaan, dan undangan.
//
// Cakupan (PRD §9.1, CORE-002, CORE-003, CORE-004):
//   - bisnis dengan jenis usaha dan modul aktif
//   - cabang dengan zona waktu dan header/footer struk
//   - terminal (register)
//   - keanggotaan (peran per bisnis) dan penugasan cabang
//
// Modul ini tidak memutuskan izin; itu tugas authz. Modul ini hanya menyediakan
// data yang dibutuhkan middleware untuk membangun tenant.Context.
package org

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

// Error domain org.
var (
	ErrBusinessNotFound = errors.New("bisnis tidak ditemukan")
	ErrBranchNotFound   = errors.New("cabang tidak ditemukan")
	ErrRegisterNotFound = errors.New("terminal tidak ditemukan")
	ErrNotMember        = errors.New("bukan anggota bisnis ini")
	ErrTypeLocked       = errors.New("jenis usaha terkunci setelah transaksi final pertama")
	ErrCodeTaken        = errors.New("kode cabang sudah dipakai")
)

// BusinessType mengikuti CHECK constraint pada businesses.business_type.
type BusinessType string

const (
	TypeRetail     BusinessType = "retail"
	TypeRestaurant BusinessType = "restaurant"
	TypePharmacy   BusinessType = "pharmacy"
)

// AllBusinessTypes adalah daftar jenis usaha yang didukung (CORE-002).
var AllBusinessTypes = []BusinessType{TypeRetail, TypeRestaurant, TypePharmacy}

// ModulesFor mengembalikan modul yang diaktifkan untuk sebuah jenis usaha.
// Satu jenis usaha = satu modul utama; modul lain (mis. apotek di retail) tidak aktif.
func ModulesFor(t BusinessType) []string {
	switch t {
	case TypeRetail:
		return []string{"retail"}
	case TypeRestaurant:
		return []string{"restaurant"}
	case TypePharmacy:
		return []string{"pharmacy"}
	default:
		return nil
	}
}

// Valid melaporkan apakah jenis usaha dikenali.
func (t BusinessType) Valid() bool {
	for _, b := range AllBusinessTypes {
		if b == t {
			return true
		}
	}
	return false
}

// Label mengembalikan nama manusiawi untuk ditampilkan.
func (t BusinessType) Label() string {
	switch t {
	case TypeRetail:
		return "Toko/Minimarket"
	case TypeRestaurant:
		return "Kafe & Restoran"
	case TypePharmacy:
		return "Apotek"
	default:
		return string(t)
	}
}

// Business adalah tenant. Semua tabel bisnis merujuk ke ID ini.
type Business struct {
	ID                   idkit.ID
	Name                 string
	Type                 BusinessType
	EnabledModules       []string
	FirstSaleCompletedAt *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// TypeLocked melaporkan apakah jenis usaha sudah tidak dapat diubah (PRD §5.3).
func (b Business) TypeLocked() bool { return b.FirstSaleCompletedAt != nil }

// Branch adalah cabang fisik. Zona waktu menentukan tanggal bisnis (§13.10).
type Branch struct {
	ID            idkit.ID
	BusinessID    idkit.ID
	Code          string
	Name          string
	Timezone      string
	Address       string
	ReceiptHeader string
	ReceiptFooter string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Register adalah terminal kasir. Shift terikat ke terminal, bukan ke pengguna.
type Register struct {
	ID         idkit.ID
	BusinessID idkit.ID
	BranchID   idkit.ID
	Name       string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Membership menghubungkan pengguna global ke bisnis dengan satu peran.
type Membership struct {
	ID             idkit.ID
	UserID         idkit.ID
	BusinessID     idkit.ID
	Role           tenant.Role
	Status         string
	PINHash        string
	PINFailCount   int
	PINLockedUntil *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ---------------------------------------------------------------------------
// Repository
// ---------------------------------------------------------------------------

// Repo menggabungkan seluruh akses data modul org.
type Repo struct {
	db database.Getter
}

// NewRepo membuat repository org.
func NewRepo(db database.Getter) *Repo { return &Repo{db: db} }

// BusinessByID mengambil bisnis berdasarkan ID.
func (r *Repo) BusinessByID(ctx context.Context, id idkit.ID) (*Business, error) {
	const q = `
SELECT id, name, business_type, enabled_modules, first_sale_completed_at, created_at, updated_at
FROM businesses WHERE id = $1`

	var b Business
	var typ string
	err := r.db.QueryRow(ctx, q, id).Scan(
		&b.ID, &b.Name, &typ, &b.EnabledModules, &b.FirstSaleCompletedAt, &b.CreatedAt, &b.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBusinessNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("org: baca bisnis: %w", err)
	}
	b.Type = BusinessType(typ)
	return &b, nil
}

// CreateBusiness membuat bisnis baru dan mengembalikan entitasnya.
// Modul aktif diisi otomatis dari jenis usaha.
func (r *Repo) CreateBusiness(ctx context.Context, name string, t BusinessType, createdBy idkit.ID) (*Business, error) {
	if !t.Valid() {
		return nil, fmt.Errorf("org: jenis usaha tidak dikenal: %q", t)
	}

	b := &Business{
		ID:             idkit.New(),
		Name:           name,
		Type:           t,
		EnabledModules: ModulesFor(t),
	}
	const q = `
INSERT INTO businesses (id, name, business_type, enabled_modules, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING created_at, updated_at`

	if err := r.db.QueryRow(ctx, q, b.ID, b.Name, string(b.Type), b.EnabledModules, createdBy).
		Scan(&b.CreatedAt, &b.UpdatedAt); err != nil {
		return nil, fmt.Errorf("org: buat bisnis: %w", err)
	}
	return b, nil
}

// UpdateBusinessType mengubah jenis usaha bila belum ada transaksi final (§5.3).
func (r *Repo) UpdateBusinessType(ctx context.Context, id idkit.ID, t BusinessType) error {
	if !t.Valid() {
		return fmt.Errorf("org: jenis usaha tidak dikenal: %q", t)
	}
	const q = `
UPDATE businesses
   SET business_type = $2, enabled_modules = $3, updated_at = now()
 WHERE id = $1
   AND first_sale_completed_at IS NULL`

	tag, err := r.db.Exec(ctx, q, id, string(t), ModulesFor(t))
	if err != nil {
		return fmt.Errorf("org: ubah jenis usaha: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Bisa karena bisnis tidak ada, atau karena sudah terkunci.
		if _, err := r.BusinessByID(ctx, id); err != nil {
			return err
		}
		return ErrTypeLocked
	}
	return nil
}

// BranchesForBusiness mengambil cabang aktif sebuah bisnis.
func (r *Repo) BranchesForBusiness(ctx context.Context, businessID idkit.ID) ([]Branch, error) {
	const q = `
SELECT id, business_id, code, name, timezone, COALESCE(address, ''),
       COALESCE(receipt_header, ''), COALESCE(receipt_footer, ''), status, created_at, updated_at
FROM branches
WHERE business_id = $1 AND status = 'active'
ORDER BY code`

	rows, err := r.db.Query(ctx, q, businessID)
	if err != nil {
		return nil, fmt.Errorf("org: baca cabang: %w", err)
	}
	defer rows.Close()

	var out []Branch
	for rows.Next() {
		var b Branch
		if err := rows.Scan(&b.ID, &b.BusinessID, &b.Code, &b.Name, &b.Timezone, &b.Address,
			&b.ReceiptHeader, &b.ReceiptFooter, &b.Status, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("org: scan cabang: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// BranchByID mengambil satu cabang.
func (r *Repo) BranchByID(ctx context.Context, businessID, branchID idkit.ID) (*Branch, error) {
	const q = `
SELECT id, business_id, code, name, timezone, COALESCE(address, ''),
       COALESCE(receipt_header, ''), COALESCE(receipt_footer, ''), status, created_at, updated_at
FROM branches WHERE business_id = $1 AND id = $2`

	var b Branch
	err := r.db.QueryRow(ctx, q, businessID, branchID).Scan(
		&b.ID, &b.BusinessID, &b.Code, &b.Name, &b.Timezone, &b.Address,
		&b.ReceiptHeader, &b.ReceiptFooter, &b.Status, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBranchNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("org: baca cabang: %w", err)
	}
	return &b, nil
}

// CreateBranch membuat cabang baru.
func (r *Repo) CreateBranch(ctx context.Context, b *Branch) error {
	if b.ID.IsZero() {
		b.ID = idkit.New()
	}
	if b.Timezone == "" {
		b.Timezone = "Asia/Jakarta"
	}
	const q = `
INSERT INTO branches (id, business_id, code, name, timezone, address, receipt_header, receipt_footer, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active')
RETURNING created_at, updated_at`

	err := r.db.QueryRow(ctx, q,
		b.ID, b.BusinessID, b.Code, b.Name, b.Timezone,
		nullable(b.Address), nullable(b.ReceiptHeader), nullable(b.ReceiptFooter),
	).Scan(&b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err, "branches_business_id_code_key") {
			return ErrCodeTaken
		}
		return fmt.Errorf("org: buat cabang: %w", err)
	}
	b.Status = "active"
	return nil
}

// RegistersForBranch mengambil terminal aktif sebuah cabang.
func (r *Repo) RegistersForBranch(ctx context.Context, businessID, branchID idkit.ID) ([]Register, error) {
	const q = `
SELECT id, business_id, branch_id, name, status, created_at, updated_at
FROM registers
WHERE business_id = $1 AND branch_id = $2 AND status = 'active'
ORDER BY name`

	rows, err := r.db.Query(ctx, q, businessID, branchID)
	if err != nil {
		return nil, fmt.Errorf("org: baca terminal: %w", err)
	}
	defer rows.Close()

	var out []Register
	for rows.Next() {
		var reg Register
		if err := rows.Scan(&reg.ID, &reg.BusinessID, &reg.BranchID, &reg.Name, &reg.Status,
			&reg.CreatedAt, &reg.UpdatedAt); err != nil {
			return nil, fmt.Errorf("org: scan terminal: %w", err)
		}
		out = append(out, reg)
	}
	return out, rows.Err()
}

// RegisterByID mengambil satu terminal.
func (r *Repo) RegisterByID(ctx context.Context, businessID, registerID idkit.ID) (*Register, error) {
	const q = `
SELECT id, business_id, branch_id, name, status, created_at, updated_at
FROM registers WHERE business_id = $1 AND id = $2`

	var reg Register
	err := r.db.QueryRow(ctx, q, businessID, registerID).Scan(
		&reg.ID, &reg.BusinessID, &reg.BranchID, &reg.Name, &reg.Status, &reg.CreatedAt, &reg.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRegisterNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("org: baca terminal: %w", err)
	}
	return &reg, nil
}

// CreateRegister membuat terminal baru pada sebuah cabang.
func (r *Repo) CreateRegister(ctx context.Context, businessID, branchID idkit.ID, name string) (*Register, error) {
	reg := &Register{
		ID:         idkit.New(),
		BusinessID: businessID,
		BranchID:   branchID,
		Name:       name,
	}
	const q = `
INSERT INTO registers (id, business_id, branch_id, name)
VALUES ($1, $2, $3, $4)
RETURNING status, created_at, updated_at`

	if err := r.db.QueryRow(ctx, q, reg.ID, businessID, branchID, name).
		Scan(&reg.Status, &reg.CreatedAt, &reg.UpdatedAt); err != nil {
		return nil, fmt.Errorf("org: buat terminal: %w", err)
	}
	return reg, nil
}

// MembershipFor mengambil keanggotaan pengguna pada sebuah bisnis.
func (r *Repo) MembershipFor(ctx context.Context, userID, businessID idkit.ID) (*Membership, error) {
	const q = `
SELECT id, user_id, business_id, role, status, COALESCE(pin_hash, ''),
       pin_fail_count, pin_locked_until, created_at, updated_at
FROM memberships
WHERE user_id = $1 AND business_id = $2`

	var m Membership
	var role string
	err := r.db.QueryRow(ctx, q, userID, businessID).Scan(
		&m.ID, &m.UserID, &m.BusinessID, &role, &m.Status, &m.PINHash,
		&m.PINFailCount, &m.PINLockedUntil, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotMember
	}
	if err != nil {
		return nil, fmt.Errorf("org: baca keanggotaan: %w", err)
	}
	m.Role = tenant.Role(role)
	return &m, nil
}

// MembershipsForUser mengambil seluruh keanggotaan aktif pengguna beserta
// nama bisnisnya, dipakai untuk pemilih bisnis di frontend (§18.1).
func (r *Repo) MembershipsForUser(ctx context.Context, userID idkit.ID) ([]MembershipSummary, error) {
	const q = `
SELECT m.id, m.business_id, m.role, b.name, b.business_type
FROM memberships m
JOIN businesses b ON b.id = m.business_id
WHERE m.user_id = $1 AND m.status = 'active'
ORDER BY b.name`

	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("org: baca daftar bisnis: %w", err)
	}
	defer rows.Close()

	var out []MembershipSummary
	for rows.Next() {
		var s MembershipSummary
		var role, typ string
		if err := rows.Scan(&s.MembershipID, &s.BusinessID, &role, &s.BusinessName, &typ); err != nil {
			return nil, fmt.Errorf("org: scan daftar bisnis: %w", err)
		}
		s.Role = tenant.Role(role)
		s.BusinessType = BusinessType(typ)
		out = append(out, s)
	}
	return out, rows.Err()
}

// MembershipSummary adalah baris ringkas untuk pemilih bisnis.
type MembershipSummary struct {
	MembershipID idkit.ID
	BusinessID   idkit.ID
	BusinessName string
	BusinessType BusinessType
	Role         tenant.Role
}

// BranchIDsForMembership mengambil cabang yang ditugaskan ke sebuah keanggotaan.
func (r *Repo) BranchIDsForMembership(ctx context.Context, membershipID idkit.ID) ([]idkit.ID, error) {
	const q = `SELECT branch_id FROM membership_branches WHERE membership_id = $1`
	rows, err := r.db.Query(ctx, q, membershipID)
	if err != nil {
		return nil, fmt.Errorf("org: baca penugasan cabang: %w", err)
	}
	defer rows.Close()

	var out []idkit.ID
	for rows.Next() {
		var id idkit.ID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
