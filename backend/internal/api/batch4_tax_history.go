package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) batch4Routes(mux *http.ServeMux) {
	for _, entity := range []string{"products", "categories"} {
		mux.Handle("GET /api/v1/"+entity+"/{id}/tax-rule", s.guard(authz.PermBusinessManage, false, s.taxAssignment))
	}
	mux.Handle("POST /api/v1/tax-rules/{id}/restore", s.guard(authz.PermBusinessManage, true, s.restoreTax))
	mux.Handle("GET /api/v1/sales/{id}/returns", s.guard(authz.PermSaleReturn, false, s.returnHistory))
	mux.Handle("GET /api/v1/sales/{id}/void", s.guard(authz.PermSaleReturn, false, s.voidDetail))
}
func (s *Server) taxAssignment(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("id tidak valid"))
		return
	}
	table := "products"
	if strings.Contains(r.Pattern, "/categories/") {
		table = "categories"
	}
	var rule *string
	var version int
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT tax_rule_id::text,version FROM `+table+` WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&rule, &version)
	if readError(w, r, err, table) {
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id.String(), "tax_rule_id": rule, "version": version})
}
func (s *Server) restoreTax(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current int
		var archived bool
		err := tx.QueryRow(ctx, `SELECT version,archived FROM scoped_tax_rules WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&current, &archived)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("pajak")
		}
		if err != nil {
			return err
		}
		if current != v || !archived {
			return errs.New(errs.CodeOptimisticLockFailure, "pajak berubah/sudah aktif", 409)
		}
		if _, err := tx.Exec(ctx, `UPDATE scoped_tax_rules SET archived=false,version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "tax_rule.restored", EntityType: "scoped_tax_rule", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id.String(), "version": v + 1, "archived": false})
}
func (s *Server) returnHistory(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	sale, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("sale_id tidak valid"))
		return
	}
	cursor, ok := pageCursor(w, r)
	if !ok {
		return
	}
	var branch idkit.ID
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT branch_id FROM sales WHERE business_id=$1 AND id=$2`, tc.BusinessID, sale).Scan(&branch)
	if readError(w, r, err, "penjualan") {
		return
	}
	if !tc.HasBranch(branch) {
		httpx.WriteError(w, r, errs.PermissionDenied(""))
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,amount_cents::text,cash_rounding_cents::text,reason,created_by::text,approved_by::text,created_at FROM sale_returns WHERE business_id=$1 AND sale_id=$2 AND id>$3 ORDER BY id LIMIT 101`, tc.BusinessID, sale, cursor)
	if readError(w, r, err, "retur") {
		return
	}
	defer rows.Close()
	type ret struct {
		ID       string    `json:"return_id"`
		Amount   string    `json:"amount_cents"`
		Rounding *string   `json:"cash_rounding_cents"`
		Reason   string    `json:"reason"`
		Actor    string    `json:"created_by"`
		Approver *string   `json:"approved_by"`
		At       time.Time `json:"created_at"`
	}
	out := []ret{}
	for rows.Next() {
		var x ret
		if readError(w, r, rows.Scan(&x.ID, &x.Amount, &x.Rounding, &x.Reason, &x.Actor, &x.Approver, &x.At), "retur") {
			return
		}
		out = append(out, x)
	}
	if readError(w, r, rows.Err(), "retur") {
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].ID
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, 200, map[string]any{"returns": out, "next_cursor": next})
}
func (s *Server) voidDetail(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	sale, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("sale_id tidak valid"))
		return
	}
	var branch idkit.ID
	var status string
	var actor, approver, reason *string
	var at *time.Time
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT branch_id,status,voided_by::text,void_approved_by::text,void_reason,voided_at FROM sales WHERE business_id=$1 AND id=$2`, tc.BusinessID, sale).Scan(&branch, &status, &actor, &approver, &reason, &at)
	if readError(w, r, err, "penjualan") {
		return
	}
	if !tc.HasBranch(branch) {
		httpx.WriteError(w, r, errs.PermissionDenied(""))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, 200, map[string]any{"sale_id": sale.String(), "status": status, "voided_by": actor, "approved_by": approver, "reason": reason, "voided_at": at})
}
