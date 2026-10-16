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

func (s *Server) customerRoutes(mux *http.ServeMux) {
	// Operational users may create/search. Erasure requires business management permission.
	mux.Handle("POST /api/v1/customers", s.guard(authz.PermSaleCreate, true, s.createCustomer))
	mux.Handle("GET /api/v1/customers", s.guard(authz.PermSaleCreate, false, s.listCustomers))
	mux.Handle("DELETE /api/v1/customers/{id}", s.guard(authz.PermBusinessManage, true, s.anonymizeCustomer))
}
func (s *Server) createCustomer(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
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
		httpx.WriteError(w, r, errs.Validation("nama wajib maksimal 200, HP 32 byte, catatan 200 karakter"))
		return
	}
	for _, c := range req.Phone {
		if !(c >= '0' && c <= '9') && c != '+' && c != ' ' && c != '-' && c != '(' && c != ')' {
			httpx.WriteError(w, r, errs.Validation("format HP tidak valid"))
			return
		}
	}
	id := idkit.New()
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO customers(id,business_id,name,phone,note,created_by) VALUES($1,$2,$3,$4,$5,$6)`, id, tc.BusinessID, req.Name, req.Phone, req.Note, tc.UserID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "customer.created", EntityType: "customer", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id.String(), "name": req.Name, "phone": req.Phone, "note": req.Note, "version": 1})
}
func (s *Server) anonymizeCustomer(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, e1 := idkit.Parse(r.PathValue("id"))
	v, e2 := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if e1 != nil || e2 != nil || v <= 0 {
		httpx.WriteError(w, r, errs.Validation("id dan If-Match wajib valid"))
		return
	}
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var version int
		var anonymized bool
		err := tx.QueryRow(ctx, `SELECT version,anonymized FROM customers WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&version, &anonymized)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("pelanggan")
		}
		if err != nil {
			return err
		}
		if anonymized {
			return nil
		}
		if version != v {
			return errs.New(errs.CodeOptimisticLockFailure, "pelanggan diubah pengguna lain", http.StatusConflict)
		}
		if _, err := tx.Exec(ctx, `UPDATE customers SET name='Pelanggan dihapus',phone='',note='',anonymized=true,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "customer.anonymized", EntityType: "customer", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
func (s *Server) listCustomers(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var cursor idkit.ID
	if raw := r.URL.Query().Get("after"); raw != "" {
		var err error
		cursor, err = idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return
		}
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 200 {
		httpx.WriteError(w, r, errs.Validation("pencarian maksimal 200 byte"))
		return
	}
	pattern := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,name,phone,note,version FROM customers WHERE business_id=$1 AND id>$2 AND NOT anonymized AND ($3='' OR name ILIKE '%'||$3||'%' OR phone ILIKE '%'||$3||'%') ORDER BY id LIMIT 101`, tc.BusinessID, cursor, pattern)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type customer struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Phone   string `json:"phone"`
		Note    string `json:"note"`
		Version int    `json:"version"`
	}
	out := []customer{}
	for rows.Next() {
		var x customer
		if err := rows.Scan(&x.ID, &x.Name, &x.Phone, &x.Note, &x.Version); err != nil {
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
	httpx.JSON(w, http.StatusOK, map[string]any{"customers": out, "next_cursor": next})
}
