// Package catalog mengelola kategori, produk, varian, barcode, dan harga jual
// (PRD CORE-007, CORE-008, CORE-011).
//
// Semua query menyertakan business_id; pemanggil mengambilnya dari tenant.Context.
package catalog

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

var (
	ErrNotFound        = errors.New("data katalog tidak ditemukan")
	ErrSKUTaken        = errors.New("SKU sudah dipakai")
	ErrCategoryTaken   = errors.New("nama kategori sudah dipakai")
	ErrCategoryInUse   = errors.New("kategori masih dipakai produk; arsipkan saja")
	ErrVersionConflict = errors.New("data sudah diubah pengguna lain; muat ulang")
)

// BarcodeTakenError menyebut varian yang sudah memakai barcode (AC CORE-008).
type BarcodeTakenError struct {
	Barcode     string
	VariantSKU  string
	ProductName string
}

func (e *BarcodeTakenError) Error() string {
	return fmt.Sprintf("barcode %s sudah dipakai oleh %s (%s)", e.Barcode, e.ProductName, e.VariantSKU)
}

type Category struct {
	ID      idkit.ID
	Name    string
	Status  string
	Version int
}

type Variant struct {
	ID               idkit.ID
	ProductID        idkit.ID
	Name             string
	SKU              string
	BaseUnit         string
	PriceCents       int64
	PriceIncludesTax bool
	AllowFractional  bool
	Attributes       map[string]string
	Status           string
	Version          int
	Barcodes         []string
}

type Product struct {
	ID         idkit.ID
	CategoryID *idkit.ID
	Name       string
	TrackStock bool
	Status     string
	Version    int
	CreatedAt  time.Time
	Variants   []Variant
}

// NewVariant adalah input varian saat membuat produk.
type NewVariant struct {
	Name             string
	SKU              string
	BaseUnit         string
	PriceCents       int64
	PriceIncludesTax bool
	AllowFractional  bool
	Attributes       map[string]string
	Barcodes         []string
}

type Repo struct{ db database.Getter }

func NewRepo(db database.Getter) *Repo { return &Repo{db: db} }

// CreateCategory membuat kategori satu level.
func (r *Repo) CreateCategory(ctx context.Context, businessID idkit.ID, name string) (*Category, error) {
	c := &Category{ID: idkit.New(), Name: strings.TrimSpace(name), Status: "active", Version: 1}
	tag, err := r.db.Exec(ctx,
		`INSERT INTO categories (id, business_id, name) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		c.ID, businessID, c.Name)
	if err != nil {
		return nil, fmt.Errorf("catalog: buat kategori: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrCategoryTaken
	}
	return c, nil
}

// ArchiveCategory mengarsipkan kategori. Kategori tanpa produk boleh dihapus
// lewat DeleteCategory; yang dipakai hanya bisa diarsipkan (CORE-007).
func (r *Repo) ArchiveCategory(ctx context.Context, businessID, id idkit.ID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE categories SET status='archived', version=version+1, updated_at=now()
		 WHERE business_id=$1 AND id=$2 AND status='active'`, businessID, id)
	if err != nil {
		return fmt.Errorf("catalog: arsip kategori: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteCategory menghapus kategori hanya bila belum dipakai produk.
func (r *Repo) DeleteCategory(ctx context.Context, businessID, id idkit.ID) error {
	var used bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM products WHERE business_id=$1 AND category_id=$2)`,
		businessID, id).Scan(&used); err != nil {
		return fmt.Errorf("catalog: cek pemakaian kategori: %w", err)
	}
	if used {
		return ErrCategoryInUse
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM categories WHERE business_id=$1 AND id=$2`, businessID, id)
	if err != nil {
		return fmt.Errorf("catalog: hapus kategori: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateProduct membuat produk beserta varian. Tanpa varian eksplisit, dibuat
// satu varian default (CORE-008). Panggil di dalam transaksi agar atomik.
func (r *Repo) CreateProduct(ctx context.Context, businessID idkit.ID, p *Product, variants []NewVariant) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("catalog: nama produk wajib diisi")
	}
	if len(variants) == 0 {
		return fmt.Errorf("catalog: minimal satu varian")
	}
	if err := ValidateCombinations(variants); err != nil {
		return err
	}
	p.ID = idkit.New()
	p.Status, p.Version = "active", 1
	if err := r.db.QueryRow(ctx,
		`INSERT INTO products (id, business_id, category_id, name, track_stock)
		 VALUES ($1,$2,$3,$4,$5) RETURNING created_at`,
		p.ID, businessID, p.CategoryID, strings.TrimSpace(p.Name), p.TrackStock).Scan(&p.CreatedAt); err != nil {
		return fmt.Errorf("catalog: buat produk: %w", err)
	}

	for _, nv := range variants {
		if nv.PriceCents < 0 {
			return fmt.Errorf("catalog: harga tidak boleh negatif")
		}
		unit := nv.BaseUnit
		if unit == "" {
			unit = "pcs"
		}
		v := Variant{
			ID: idkit.New(), ProductID: p.ID, Name: nv.Name, SKU: strings.TrimSpace(nv.SKU),
			BaseUnit: unit, PriceCents: nv.PriceCents, PriceIncludesTax: nv.PriceIncludesTax, AllowFractional: nv.AllowFractional,
			Status: "active", Version: 1, Attributes: nv.Attributes,
		}
		if v.Attributes == nil {
			v.Attributes = map[string]string{}
		}
		tag, err := r.db.Exec(ctx,
			`INSERT INTO variants (id, business_id, product_id, name, sku, base_unit, price_cents, price_includes_tax, allow_fractional, attributes)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING`,
			v.ID, businessID, p.ID, v.Name, v.SKU, v.BaseUnit, v.PriceCents, v.PriceIncludesTax, v.AllowFractional, v.Attributes)
		if err != nil {
			return fmt.Errorf("catalog: buat varian: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrSKUTaken
		}
		for _, bc := range nv.Barcodes {
			if err := r.addBarcode(ctx, businessID, v.ID, bc); err != nil {
				return err
			}
			v.Barcodes = append(v.Barcodes, strings.TrimSpace(bc))
		}
		p.Variants = append(p.Variants, v)
	}
	return nil
}

func (r *Repo) addBarcode(ctx context.Context, businessID, variantID idkit.ID, barcode string) error {
	barcode = strings.TrimSpace(barcode)
	tag, err := r.db.Exec(ctx,
		`INSERT INTO variant_barcodes (business_id, barcode, variant_id) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
		businessID, barcode, variantID)
	if err != nil {
		return fmt.Errorf("catalog: tambah barcode: %w", err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	// Konflik: sebut pemilik barcode agar pesan error berguna.
	e := &BarcodeTakenError{Barcode: barcode}
	if err := r.db.QueryRow(ctx,
		`SELECT v.sku, p.name FROM variant_barcodes b
		 JOIN variants v ON v.business_id=b.business_id AND v.id=b.variant_id
		 JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id
		 WHERE b.business_id=$1 AND b.barcode=$2`, businessID, barcode).Scan(&e.VariantSKU, &e.ProductName); err != nil {
		return fmt.Errorf("catalog: cari pemilik barcode: %w", err)
	}
	return e
}

// VariantByBarcode mencari varian dari barcode (jalur scan kasir).
func (r *Repo) VariantByBarcode(ctx context.Context, businessID idkit.ID, barcode string) (*Variant, error) {
	var v Variant
	err := r.db.QueryRow(ctx,
		`SELECT v.id, v.product_id, v.name, v.sku, v.base_unit, v.price_cents, v.price_includes_tax, v.allow_fractional, v.attributes, v.status, v.version
		 FROM variant_barcodes b
		 JOIN variants v ON v.business_id=b.business_id AND v.id=b.variant_id
		 WHERE b.business_id=$1 AND b.barcode=$2`, businessID, strings.TrimSpace(barcode)).
		Scan(&v.ID, &v.ProductID, &v.Name, &v.SKU, &v.BaseUnit, &v.PriceCents, &v.PriceIncludesTax, &v.AllowFractional, &v.Attributes, &v.Status, &v.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("catalog: cari barcode: %w", err)
	}
	return &v, nil
}

// UpdatePrice mengubah harga dengan optimistic locking (version harus cocok).
// Riwayat harga masuk audit oleh pemanggil (CORE-011).
func (r *Repo) UpdatePrice(ctx context.Context, businessID, variantID idkit.ID, version int, priceCents int64) (int, error) {
	if priceCents < 0 {
		return 0, fmt.Errorf("catalog: harga tidak boleh negatif")
	}
	var newVersion int
	err := r.db.QueryRow(ctx,
		`UPDATE variants SET price_cents=$4, version=version+1, updated_at=now()
		 WHERE business_id=$1 AND id=$2 AND version=$3 RETURNING version`,
		businessID, variantID, version, priceCents).Scan(&newVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if e := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM variants WHERE business_id=$1 AND id=$2)`,
			businessID, variantID).Scan(&exists); e != nil {
			return 0, fmt.Errorf("catalog: cek varian: %w", e)
		}
		if !exists {
			return 0, ErrNotFound
		}
		return 0, ErrVersionConflict
	}
	if err != nil {
		return 0, fmt.Errorf("catalog: ubah harga: %w", err)
	}
	return newVersion, nil
}
