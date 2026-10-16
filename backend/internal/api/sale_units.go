package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) unitRoutes(mux *http.ServeMux) {
	mux.Handle("PUT /api/v1/purchase-units/{id}", s.guard(authz.PermProductManage, true, s.editUnit))
	mux.Handle("GET /api/v1/variants/{id}/unit-defaults", s.guard(authz.PermProductView, false, s.getUnitDefaults))
	mux.Handle("PUT /api/v1/variants/{id}/unit-defaults", s.guard(authz.PermProductManage, true, s.putUnitDefaults))
	mux.Handle("PUT /api/v1/purchase-units/{id}/sale-price", s.guard(authz.PermProductManage, true, s.unitPrice))
	mux.Handle("POST /api/v1/purchase-units/{id}/barcodes", s.guard(authz.PermProductManage, true, s.unitBarcode))
	mux.Handle("GET /api/v1/catalog/barcode", s.guard(authz.PermProductView, false, s.resolveBarcode))
	mux.Handle("GET /api/v1/variants/{id}/units", s.guard(authz.PermProductView, false, s.listUnits))
}
func (s *Server) unitPrice(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("unit_id tidak valid"))
		return
	}
	var req struct {
		Price string `json:"sale_price_cents"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	price, err := money.ParseCents(req.Price)
	if err != nil || price < 0 {
		httpx.WriteError(w, r, errs.Validation("harga tidak valid"))
		return
	}
	var expected *int
	if raw := r.Header.Get("If-Match"); raw != "" {
		v, e := strconv.Atoi(strings.Trim(raw, `"`))
		if e != nil || v <= 0 {
			httpx.WriteError(w, r, errs.Validation("If-Match tidak valid"))
			return
		}
		expected = &v
	}
	// shortcut: price version optional for old clients; require after frontend header migration.
	var version int
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var old *int64
		err := tx.QueryRow(ctx, `SELECT sale_price_cents,version FROM purchase_units WHERE business_id=$1 AND id=$2 AND NOT archived FOR UPDATE`, tc.BusinessID, id).Scan(&old, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("satuan")
		}
		if err != nil {
			return err
		}
		if expected != nil && *expected != version {
			return errs.New(errs.CodeOptimisticLockFailure, "harga satuan diubah pengguna lain", http.StatusConflict)
		}
		if _, err := tx.Exec(ctx, `UPDATE purchase_units SET version=version+1,sale_price_cents=$3 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, int64(price)); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "unit.price_changed", EntityType: "unit", EntityID: id.String(), Before: map[string]any{"sale_price_cents": old}, After: map[string]string{"sale_price_cents": req.Price}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok", "version": version + 1})
}
func (s *Server) unitBarcode(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("unit_id tidak valid"))
		return
	}
	var req struct {
		Barcode string `json:"barcode"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	barcode := strings.TrimSpace(req.Barcode)
	if barcode == "" || len(barcode) > 200 {
		httpx.WriteError(w, r, errs.Validation("barcode wajib, maksimal 200 byte"))
		return
	}
	tag, err := s.pool.Underlying().Exec(r.Context(), `INSERT INTO variant_barcodes(business_id,barcode,variant_id,unit_id) SELECT business_id,$3,variant_id,id FROM purchase_units WHERE business_id=$1 AND id=$2 AND NOT archived ON CONFLICT DO NOTHING`, tc.BusinessID, id, barcode)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if tag.RowsAffected() != 1 {
		httpx.WriteError(w, r, errs.Conflict("barcode sudah dipakai atau satuan tidak ditemukan"))
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"barcode": barcode, "unit_id": id.String()})
}
func (s *Server) resolveBarcode(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	barcode := strings.TrimSpace(r.URL.Query().Get("barcode"))
	if barcode == "" {
		httpx.WriteError(w, r, errs.Validation("barcode wajib diisi"))
		return
	}
	var variant, sku, unit, factor, price string
	var uid *string
	var fractional bool
	err := s.pool.Underlying().QueryRow(r.Context(), `SELECT v.id::text,v.sku,b.unit_id::text,COALESCE(u.name,v.base_unit),COALESCE(u.factor_milli,1000)::text,COALESCE(u.sale_price_cents,v.price_cents)::text,v.allow_fractional
 FROM variant_barcodes b JOIN variants v ON v.business_id=b.business_id AND v.id=b.variant_id
 JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id
 LEFT JOIN purchase_units u ON u.business_id=b.business_id AND u.id=b.unit_id
 WHERE b.business_id=$1 AND b.barcode=$2 AND v.status='active' AND p.status='active' AND (b.unit_id IS NULL OR (NOT u.archived AND u.sale_price_cents IS NOT NULL))`, tc.BusinessID, barcode).Scan(&variant, &sku, &uid, &unit, &factor, &price, &fractional)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, errs.NotFound("barcode aktif"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"variant_id": variant, "sku": sku, "unit_id": uid, "unit_name": unit, "factor_milli": factor, "price_cents": price, "allow_fractional": fractional})
}
func (s *Server) listUnits(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("variant_id tidak valid"))
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,name,factor_milli::text,sale_price_cents::text,used_at IS NOT NULL,version FROM purchase_units WHERE business_id=$1 AND variant_id=$2 AND NOT archived ORDER BY name,id`, tc.BusinessID, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type unit struct {
		ID      string  `json:"unit_id"`
		Name    string  `json:"name"`
		Factor  string  `json:"factor_milli"`
		Price   *string `json:"sale_price_cents"`
		Locked  bool    `json:"factor_locked"`
		Version int     `json:"version"`
	}
	out := []unit{}
	for rows.Next() {
		var u unit
		if err := rows.Scan(&u.ID, &u.Name, &u.Factor, &u.Price, &u.Locked, &u.Version); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"units": out})
}
