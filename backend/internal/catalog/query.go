package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/storepos/storepos/internal/platform/idkit"
)

// Hit adalah baris hasil pencarian kasir: varian beserta nama produknya.
type Hit struct {
	Variant
	ProductName string
	TrackStock  bool
}

// ListCategories mengambil kategori aktif.
func (r *Repo) ListCategories(ctx context.Context, businessID idkit.ID) ([]Category, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, status, version FROM categories
		 WHERE business_id=$1 AND status='active' ORDER BY lower(name)`, businessID)
	if err != nil {
		return nil, fmt.Errorf("catalog: daftar kategori: %w", err)
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Status, &c.Version); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// Search mencari varian aktif berdasarkan nama produk, SKU (sebagian), atau
// barcode (persis). Kosong = semua, terbatas oleh limit.
//
// shortcut: substring memakai ILIKE tanpa trigram; tambah indeks bila pengukuran HTTP 50k
// melewati p95 300ms. Barcode dipilih sekali lewat indeks tenant+barcode, bukan lookup per varian.
func (r *Repo) Search(ctx context.Context, businessID idkit.ID, q string, limit int) ([]Hit, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q = strings.TrimSpace(q)
	rows, err := r.db.Query(ctx, `
SELECT v.id, v.product_id, v.name, v.sku, v.base_unit, v.price_cents, v.price_includes_tax, v.allow_fractional, v.attributes,
       v.status, v.version, p.name, p.track_stock
FROM variants v
JOIN products p ON p.business_id = v.business_id AND p.id = v.product_id
WHERE v.business_id = $1 AND v.status = 'active' AND p.status = 'active'
  AND ($2 = '' OR p.name ILIKE '%' || $3 || '%' OR v.sku ILIKE '%' || $3 || '%'
       OR v.id IN (SELECT b.variant_id FROM variant_barcodes b
                   WHERE b.business_id = $1 AND b.barcode = $2))
ORDER BY p.name, v.sku
LIMIT $4`, businessID, q, likeEscaper.Replace(q), limit)
	if err != nil {
		return nil, fmt.Errorf("catalog: cari: %w", err)
	}
	defer rows.Close()
	var out []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.ID, &h.ProductID, &h.Variant.Name, &h.SKU, &h.BaseUnit, &h.PriceCents,
			&h.PriceIncludesTax, &h.AllowFractional, &h.Attributes, &h.Status, &h.Version, &h.ProductName, &h.TrackStock); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// VariantByID mengambil satu varian milik bisnis.
func (r *Repo) VariantByID(ctx context.Context, businessID, id idkit.ID) (*Variant, error) {
	var v Variant
	err := r.db.QueryRow(ctx,
		`SELECT id, product_id, name, sku, base_unit, price_cents, price_includes_tax, allow_fractional, attributes, status, version
		 FROM variants WHERE business_id=$1 AND id=$2`, businessID, id).
		Scan(&v.ID, &v.ProductID, &v.Name, &v.SKU, &v.BaseUnit, &v.PriceCents, &v.PriceIncludesTax, &v.AllowFractional, &v.Attributes, &v.Status, &v.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("catalog: baca varian: %w", err)
	}
	return &v, nil
}
