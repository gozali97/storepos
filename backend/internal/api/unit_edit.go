package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) editUnit(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, e1 := idkit.Parse(r.PathValue("id"))
	v, e2 := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if e1 != nil || e2 != nil || v <= 0 {
		httpx.WriteError(w, r, errs.Validation("id/If-Match wajib valid"))
		return
	}
	var req struct {
		Name     string `json:"name"`
		Factor   string `json:"factor_milli"`
		Archived bool   `json:"archived"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	factor, err := money.ParseMilli(req.Factor)
	if err != nil || factor <= 0 || req.Name == "" || utf8.RuneCountInString(req.Name) > 100 {
		httpx.WriteError(w, r, errs.Validation("nama/faktor satuan tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var variant idkit.ID
		err := tx.QueryRow(ctx, `SELECT variant_id FROM purchase_units WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&variant)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("satuan")
		}
		if err != nil {
			return err
		}
		// Same order as setting defaults: variant then unit, avoiding an archive/default race.
		var defaults bool
		if err := tx.QueryRow(ctx, `SELECT COALESCE(default_purchase_unit_id=$2 OR default_sale_unit_id=$2,false) FROM variants WHERE business_id=$1 AND id=$3 FOR UPDATE`, tc.BusinessID, id, variant).Scan(&defaults); err != nil {
			return err
		}
		var oldName string
		var oldFactor int64
		var used *time.Time
		var current int
		var oldArchived bool
		if err := tx.QueryRow(ctx, `SELECT name,factor_milli,used_at,version,archived FROM purchase_units WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&oldName, &oldFactor, &used, &current, &oldArchived); err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "satuan diubah pengguna lain", http.StatusConflict)
		}
		if used != nil && oldFactor != int64(factor) {
			return errs.Conflict("faktor sudah dipakai; buat satuan baru dan arsipkan yang lama")
		}
		if req.Archived && defaults {
			return errs.Conflict("ubah satuan default sebelum mengarsipkan")
		}
		if _, err := tx.Exec(ctx, `UPDATE purchase_units SET name=$3,factor_milli=$4,archived=$5,version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Name, int64(factor), req.Archived); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "unit.updated", EntityType: "unit", EntityID: id.String(), Before: map[string]any{"name": oldName, "factor_milli": oldFactor, "archived": oldArchived}, After: map[string]any{"name": req.Name, "factor_milli": req.Factor, "archived": req.Archived}})
	})
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23505" {
		httpx.WriteError(w, r, errs.Conflict("nama satuan sudah dipakai varian ini"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"unit_id": id.String(), "version": v + 1, "archived": req.Archived})
}
