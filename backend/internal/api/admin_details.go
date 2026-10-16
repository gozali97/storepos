package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
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

func (s *Server) adminDetailRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/customers/{id}", s.guard(authz.PermSaleCreate, false, s.customerDetail))
	mux.Handle("PUT /api/v1/customers/{id}", s.guard(authz.PermSaleCreate, true, s.editCustomer))
	mux.Handle("GET /api/v1/suppliers/{id}", s.guard(authz.PermSupplierManage, false, s.supplierDetail))
	mux.Handle("GET /api/v1/branches/{id}/registers", s.guard(authz.PermBranchView, false, s.listRegisters))
	mux.Handle("GET /api/v1/settings/business-profile", s.guard(authz.PermBusinessManage, false, s.businessProfile))
	mux.Handle("PUT /api/v1/settings/business-profile", s.guard(authz.PermBusinessManage, true, s.businessProfile))
	mux.Handle("DELETE /api/v1/categories/{id}", s.guard(authz.PermProductManage, true, s.archiveCategory))
}

type customerDetailDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Note       string `json:"note"`
	Version    int    `json:"version"`
	Anonymized bool   `json:"anonymized"`
}

func (s *Server) customerDetail(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("id tidak valid"))
		return
	}
	var x customerDetailDTO
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT id::text,name,phone,note,version,anonymized FROM customers WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&x.ID, &x.Name, &x.Phone, &x.Note, &x.Version, &x.Anonymized)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, errs.NotFound("pelanggan"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, x)
}
func (s *Server) editCustomer(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
		Note  string `json:"note"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 200 || len(req.Phone) > 32 || utf8.RuneCountInString(req.Note) > 200 {
		httpx.WriteError(w, r, errs.Validation("nama/HP/catatan tidak valid"))
		return
	}
	for _, c := range req.Phone {
		if !(c >= '0' && c <= '9') && c != '+' && c != ' ' && c != '-' && c != '(' && c != ')' {
			httpx.WriteError(w, r, errs.Validation("HP tidak valid"))
			return
		}
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current int
		var anon bool
		err := tx.QueryRow(ctx, `SELECT version,anonymized FROM customers WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&current, &anon)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("pelanggan")
		}
		if err != nil {
			return err
		}
		if anon {
			return errs.Conflict("pelanggan sudah dianonimkan")
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "pelanggan berubah", http.StatusConflict)
		}
		if _, err := tx.Exec(ctx, `UPDATE customers SET name=$3,phone=$4,note=$5,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Name, req.Phone, req.Note); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "customer.updated", EntityType: "customer", EntityID: id.String(), After: map[string]int{"version": v + 1}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id.String(), "version": v + 1})
}
func (s *Server) supplierDetail(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("id tidak valid"))
		return
	}
	var x supplierDTO
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT id::text,name,contact,address,note,archived,version FROM suppliers WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&x.ID, &x.Name, &x.Contact, &x.Address, &x.Note, &x.Archived, &x.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, errs.NotFound("supplier"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, x)
}
func (s *Server) listRegisters(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	branch, ok := branchOf(w, r, tc, r.PathValue("id"))
	if !ok {
		return
	}
	q := r.URL.Query()
	status := q.Get("status")
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "inactive" && status != "archived" && status != "all" {
		httpx.WriteError(w, r, errs.Validation("status tidak valid"))
		return
	}
	var cursor idkit.ID
	if q.Get("after") != "" {
		id, err := idkit.Parse(q.Get("after"))
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return
		}
		cursor = id
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,name,status,version FROM registers WHERE business_id=$1 AND branch_id=$2 AND ($3='all' OR status=$3) AND id>$4 ORDER BY id LIMIT 101`, tc.BusinessID, branch, status, cursor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type reg struct {
		ID      string `json:"register_id"`
		Name    string `json:"name"`
		Status  string `json:"status"`
		Version int    `json:"version"`
	}
	out := []reg{}
	for rows.Next() {
		var x reg
		if err := rows.Scan(&x.ID, &x.Name, &x.Status, &x.Version); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].ID
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"registers": out, "next_cursor": next})
}
func (s *Server) businessProfile(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	if r.Method == http.MethodGet {
		var name string
		var version int
		if err := s.pool.Underlying().QueryRow(r.Context(), `SELECT name,org_version FROM businesses WHERE id=$1`, tc.BusinessID).Scan(&name, &version); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"name": name, "version": version})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	v, err := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if err != nil || v <= 0 || req.Name == "" || utf8.RuneCountInString(req.Name) > 200 {
		httpx.WriteError(w, r, errs.Validation("nama/If-Match tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current int
		var old string
		if err := tx.QueryRow(ctx, `SELECT name,org_version FROM businesses WHERE id=$1 FOR UPDATE`, tc.BusinessID).Scan(&old, &current); err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "bisnis berubah", http.StatusConflict)
		}
		if _, err := tx.Exec(ctx, `UPDATE businesses SET name=$2,org_version=org_version+1,updated_at=now() WHERE id=$1`, tc.BusinessID, req.Name); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "business.profile_changed", EntityType: "business", EntityID: tc.BusinessID.String(), Before: map[string]string{"name": old}, After: map[string]string{"name": req.Name}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"name": req.Name, "version": v + 1})
}
func (s *Server) archiveCategory(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current int
		var status string
		err := tx.QueryRow(ctx, `SELECT version,status FROM categories WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&current, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("kategori")
		}
		if err != nil {
			return err
		}
		if current != v || status != "active" {
			return errs.New(errs.CodeOptimisticLockFailure, "kategori berubah atau diarsipkan", http.StatusConflict)
		}
		if _, err := tx.Exec(ctx, `UPDATE categories SET status='archived',version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "category.archived", EntityType: "category", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id.String(), "archived": true, "version": v + 1})
}
