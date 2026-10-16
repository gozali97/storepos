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

type supplierDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Contact  string `json:"contact"`
	Address  string `json:"address"`
	Note     string `json:"note"`
	Archived bool   `json:"archived"`
	Version  int    `json:"version"`
}

func (s *Server) supplierRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/suppliers", s.guard(authz.PermSupplierManage, true, s.saveSupplier))
	mux.Handle("PUT /api/v1/suppliers/{id}", s.guard(authz.PermSupplierManage, true, s.saveSupplier))
	mux.Handle("DELETE /api/v1/suppliers/{id}", s.guard(authz.PermSupplierManage, true, s.deleteSupplier))
	mux.Handle("GET /api/v1/suppliers", s.guard(authz.PermSupplierManage, false, s.listSuppliers))
}
func validSupplier(x supplierDTO) bool {
	return strings.TrimSpace(x.Name) != "" && utf8.RuneCountInString(x.Name) <= 200 && utf8.RuneCountInString(x.Contact) <= 200 && utf8.RuneCountInString(x.Address) <= 1000 && utf8.RuneCountInString(x.Note) <= 1000
}
func supplierVersion(r *http.Request) (int, error) {
	v, err := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if err != nil || v <= 0 {
		return 0, errs.Validation("If-Match versi positif wajib diisi")
	}
	return v, nil
}
func (s *Server) saveSupplier(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		Name     string `json:"name"`
		Contact  string `json:"contact"`
		Address  string `json:"address"`
		Note     string `json:"note"`
		Archived bool   `json:"archived"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	x := supplierDTO{Name: strings.TrimSpace(req.Name), Contact: req.Contact, Address: req.Address, Note: req.Note, Archived: req.Archived, Version: 1}
	if !validSupplier(x) {
		httpx.WriteError(w, r, errs.Validation("nama wajib; batas nama/kontak 200 dan alamat/catatan 1000 karakter"))
		return
	}
	id := idkit.New()
	v := 0
	if r.Method == http.MethodPut {
		var err error
		id, err = idkit.Parse(r.PathValue("id"))
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("supplier_id tidak valid"))
			return
		}
		v, err = supplierVersion(r)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	x.ID = id.String()
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if v == 0 {
			_, err := tx.Exec(ctx, `INSERT INTO suppliers(id,business_id,name,contact,address,note,archived,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, tc.BusinessID, x.Name, x.Contact, x.Address, x.Note, x.Archived, tc.UserID)
			if err != nil {
				return err
			}
		} else {
			var current int
			err := tx.QueryRow(ctx, `SELECT version FROM suppliers WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&current)
			if errors.Is(err, pgx.ErrNoRows) {
				return errs.NotFound("supplier")
			}
			if err != nil {
				return err
			}
			if current != v {
				return errs.New(errs.CodeOptimisticLockFailure, "supplier diubah pengguna lain", http.StatusConflict)
			}
			if _, err := tx.Exec(ctx, `UPDATE suppliers SET name=$3,contact=$4,address=$5,note=$6,archived=$7,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, x.Name, x.Contact, x.Address, x.Note, x.Archived); err != nil {
				return err
			}
			x.Version = current + 1
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "supplier.saved", EntityType: "supplier", EntityID: x.ID, After: map[string]any{"version": x.Version, "archived": x.Archived}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	code := http.StatusOK
	if v == 0 {
		code = http.StatusCreated
	}
	httpx.JSON(w, code, x)
}
func (s *Server) deleteSupplier(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("supplier_id tidak valid"))
		return
	}
	v, err := supplierVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current int
		err := tx.QueryRow(ctx, `SELECT version FROM suppliers WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("supplier")
		}
		if err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "supplier diubah pengguna lain", http.StatusConflict)
		}
		var used bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM goods_receipts WHERE business_id=$1 AND supplier_id=$2) OR EXISTS(SELECT 1 FROM pharmacy_batch_receipts WHERE business_id=$1 AND supplier_id=$2)`, tc.BusinessID, id).Scan(&used); err != nil {
			return err
		}
		if used {
			return errs.Conflict("supplier sudah dipakai; arsipkan, jangan hapus")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM suppliers WHERE business_id=$1 AND id=$2`, tc.BusinessID, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "supplier.deleted", EntityType: "supplier", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
func (s *Server) listSuppliers(w http.ResponseWriter, r *http.Request) {
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
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,name,contact,address,note,archived,version FROM suppliers WHERE business_id=$1 AND id>$2 AND ($3='' OR name ILIKE '%'||$3||'%' OR contact ILIKE '%'||$3||'%') ORDER BY id LIMIT 101`, tc.BusinessID, cursor, pattern)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	out := []supplierDTO{}
	for rows.Next() {
		var x supplierDTO
		if err := rows.Scan(&x.ID, &x.Name, &x.Contact, &x.Address, &x.Note, &x.Archived, &x.Version); err != nil {
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
	httpx.JSON(w, http.StatusOK, map[string]any{"suppliers": out, "next_cursor": next})
}
