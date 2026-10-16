package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) scopedTaxRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/tax-rules", s.guard(authz.PermBusinessManage, true, s.saveScopedTax))
	mux.Handle("PUT /api/v1/tax-rules/{id}", s.guard(authz.PermBusinessManage, true, s.saveScopedTax))
	mux.Handle("DELETE /api/v1/tax-rules/{id}", s.guard(authz.PermBusinessManage, true, s.archiveScopedTax))
	mux.Handle("GET /api/v1/tax-rules", s.guard(authz.PermBusinessManage, false, s.listScopedTaxes))
	mux.Handle("GET /api/v1/tax-rules/{id}", s.guard(authz.PermBusinessManage, false, s.scopedTaxDetail))
	for _, entity := range []string{"products", "categories"} {
		mux.Handle("PUT /api/v1/"+entity+"/{id}/tax-rule", s.guard(authz.PermBusinessManage, true, s.assignScopedTax))
	}
}

type scopedTax struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Rate     int    `json:"rate_bps"`
	Mode     string `json:"mode"`
	Archived bool   `json:"archived"`
	Version  int    `json:"version"`
}

func (s *Server) saveScopedTax(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		Name string `json:"name"`
		Rate int    `json:"rate_bps"`
		Mode string `json:"mode"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 100 || req.Rate < 0 || req.Rate > 10000 || (req.Mode != "exclusive" && req.Mode != "inclusive") {
		httpx.WriteError(w, r, errs.Validation("aturan pajak tidak valid"))
		return
	}
	id := idkit.New()
	v := 0
	if r.Method == "PUT" {
		var err error
		id, v, err = countVersion(r)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if v == 0 {
			if _, err := tx.Exec(ctx, `INSERT INTO scoped_tax_rules(id,business_id,name,rate_bps,mode) VALUES($1,$2,$3,$4,$5)`, id, tc.BusinessID, req.Name, req.Rate, req.Mode); err != nil {
				return err
			}
		} else {
			var current int
			var archived bool
			err := tx.QueryRow(ctx, `SELECT version,archived FROM scoped_tax_rules WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&current, &archived)
			if errors.Is(err, pgx.ErrNoRows) {
				return errs.NotFound("pajak")
			}
			if err != nil {
				return err
			}
			if archived || current != v {
				return errs.New(errs.CodeOptimisticLockFailure, "pajak berubah/diarsipkan", http.StatusConflict)
			}
			if _, err := tx.Exec(ctx, `UPDATE scoped_tax_rules SET name=$3,rate_bps=$4,mode=$5,version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Name, req.Rate, req.Mode); err != nil {
				return err
			}
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "tax_rule.saved", EntityType: "scoped_tax_rule", EntityID: id.String(), After: map[string]any{"name": req.Name, "rate_bps": req.Rate, "mode": req.Mode}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	code := 200
	next := v + 1
	if v == 0 {
		code = 201
		next = 1
	}
	httpx.JSON(w, code, map[string]any{"id": id.String(), "version": next})
}
func (s *Server) scopedTaxDetail(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("id tidak valid"))
		return
	}
	var x scopedTax
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT id::text,name,rate_bps,mode,archived,version FROM scoped_tax_rules WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&x.ID, &x.Name, &x.Rate, &x.Mode, &x.Archived, &x.Version)
	if readError(w, r, err, "pajak") {
		return
	}
	httpx.JSON(w, 200, x)
}
func (s *Server) listScopedTaxes(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	cursor, ok := pageCursor(w, r)
	if !ok {
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,name,rate_bps,mode,archived,version FROM scoped_tax_rules WHERE business_id=$1 AND id>$2 ORDER BY id LIMIT 101`, tc.BusinessID, cursor)
	if readError(w, r, err, "pajak") {
		return
	}
	defer rows.Close()
	out := []scopedTax{}
	for rows.Next() {
		var x scopedTax
		if readError(w, r, rows.Scan(&x.ID, &x.Name, &x.Rate, &x.Mode, &x.Archived, &x.Version), "pajak") {
			return
		}
		out = append(out, x)
	}
	if readError(w, r, rows.Err(), "pajak") {
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].ID
	}
	httpx.JSON(w, 200, map[string]any{"rules": out, "next_cursor": next})
}
func (s *Server) archiveScopedTax(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current int
		err := tx.QueryRow(ctx, `SELECT version FROM scoped_tax_rules WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("pajak")
		}
		if err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "pajak berubah", 409)
		}
		var used bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE business_id=$1 AND tax_rule_id=$2) OR EXISTS(SELECT 1 FROM categories WHERE business_id=$1 AND tax_rule_id=$2)`, tc.BusinessID, id).Scan(&used); err != nil {
			return err
		}
		if used {
			return errs.Conflict("lepas semua penugasan sebelum arsip pajak")
		}
		if _, err := tx.Exec(ctx, `UPDATE scoped_tax_rules SET archived=true,version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "tax_rule.archived", EntityType: "scoped_tax_rule", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id.String(), "version": v + 1})
}
func (s *Server) assignScopedTax(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Rule *string `json:"tax_rule_id"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	table := "products"
	if strings.Contains(r.Pattern, "/categories/") {
		table = "categories"
	}
	var rule *idkit.ID
	if req.Rule != nil {
		x, err := idkit.Parse(*req.Rule)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("tax_rule_id tidak valid"))
			return
		}
		rule = &x
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if rule != nil {
			var archived bool
			err := tx.QueryRow(ctx, `SELECT archived FROM scoped_tax_rules WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, *rule).Scan(&archived)
			if errors.Is(err, pgx.ErrNoRows) {
				return errs.NotFound("pajak")
			}
			if err != nil {
				return err
			}
			if archived {
				return errs.Conflict("pajak diarsipkan")
			}
		}
		var current int
		err := tx.QueryRow(ctx, `SELECT version FROM `+table+` WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound(table)
		}
		if err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "master berubah", 409)
		}
		if _, err := tx.Exec(ctx, `UPDATE `+table+` SET tax_rule_id=$3,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, rule); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "tax_rule.assigned", EntityType: table, EntityID: id.String(), After: map[string]any{"tax_rule_id": req.Rule}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id.String(), "version": v + 1, "tax_rule_id": req.Rule})
}
