package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) getUnitDefaults(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("variant_id tidak valid"))
		return
	}
	var purchase, sale *string
	var version int
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT default_purchase_unit_id::text,default_sale_unit_id::text,unit_defaults_version FROM variants WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&purchase, &sale, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, errs.NotFound("varian"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"variant_id": id.String(), "default_purchase_unit_id": purchase, "default_sale_unit_id": sale, "version": version})
}
func (s *Server) putUnitDefaults(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, e1 := idkit.Parse(r.PathValue("id"))
	v, e2 := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if e1 != nil || e2 != nil || v <= 0 {
		httpx.WriteError(w, r, errs.Validation("id/If-Match wajib valid"))
		return
	}
	var req struct {
		Purchase *string `json:"default_purchase_unit_id"`
		Sale     *string `json:"default_sale_unit_id"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var purchase, sale *idkit.ID
	for i, raw := range []*string{req.Purchase, req.Sale} {
		if raw != nil {
			u, err := idkit.Parse(*raw)
			if err != nil {
				httpx.WriteError(w, r, errs.Validation("unit_id tidak valid"))
				return
			}
			if i == 0 {
				purchase = &u
			} else {
				sale = &u
			}
		}
	}
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current int
		var oldPurchase, oldSale *string
		err := tx.QueryRow(ctx, `SELECT unit_defaults_version,default_purchase_unit_id::text,default_sale_unit_id::text FROM variants WHERE business_id=$1 AND id=$2 AND status<>'archived' FOR UPDATE`, tc.BusinessID, id).Scan(&current, &oldPurchase, &oldSale)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("varian")
		}
		if err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "satuan default diubah pengguna lain", http.StatusConflict)
		}
		for i, u := range []*idkit.ID{purchase, sale} {
			if u == nil {
				continue
			}
			var price *int64
			err := tx.QueryRow(ctx, `SELECT sale_price_cents FROM purchase_units WHERE business_id=$1 AND variant_id=$2 AND id=$3 AND NOT archived FOR SHARE`, tc.BusinessID, id, *u).Scan(&price)
			if errors.Is(err, pgx.ErrNoRows) {
				return errs.Validation("satuan default bukan milik varian atau diarsipkan")
			}
			if err != nil {
				return err
			}
			if i == 1 && price == nil {
				return errs.Validation("satuan jual default harus memiliki harga jual")
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE variants SET default_purchase_unit_id=$3,default_sale_unit_id=$4,unit_defaults_version=unit_defaults_version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, purchase, sale); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "variant.unit_defaults_changed", EntityType: "variant", EntityID: id.String(), Before: map[string]any{"purchase": oldPurchase, "sale": oldSale}, After: map[string]any{"purchase": req.Purchase, "sale": req.Sale}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"variant_id": id.String(), "default_purchase_unit_id": req.Purchase, "default_sale_unit_id": req.Sale, "version": v + 1})
}
