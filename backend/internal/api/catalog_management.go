package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) catalogManagementRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/categories/{id}", s.guard(authz.PermProductView, false, s.categoryDetail))
	mux.Handle("PUT /api/v1/categories/{id}", s.guard(authz.PermProductManage, true, s.editCategory))
	mux.Handle("POST /api/v1/categories/{id}/restore", s.guard(authz.PermProductManage, true, s.editCategory))
	mux.Handle("GET /api/v1/products/{id}", s.guard(authz.PermProductView, false, s.productDetail))
	mux.Handle("PUT /api/v1/products/{id}", s.guard(authz.PermProductManage, true, s.editProduct))
	mux.Handle("PATCH /api/v1/products/{id}/status", s.guard(authz.PermProductManage, true, s.productStatus))
	mux.Handle("PATCH /api/v1/variants/{id}/identity", s.guard(authz.PermProductManage, true, s.editVariantIdentity))
	mux.Handle("PATCH /api/v1/variants/{id}/fractional", s.guard(authz.PermProductManage, true, s.editVariantFractional))
	mux.Handle("GET /api/v1/variants/{id}/barcodes", s.guard(authz.PermProductView, false, s.listVariantBarcodes))
	mux.Handle("DELETE /api/v1/variants/{id}/barcodes", s.guard(authz.PermProductManage, true, s.deleteVariantBarcode))
}
func catalogManagementError(err error) error {
	var p *pgconn.PgError
	if errors.As(err, &p) && p.Code == "23505" {
		return errs.Conflict("nama atau SKU sudah dipakai")
	}
	return err
}
func lockCatalogVersion(ctx context.Context, tx database.Tx, biz, id idkit.ID, table string, v int) error {
	var current int
	err := tx.QueryRow(ctx, `SELECT version FROM `+table+` WHERE business_id=$1 AND id=$2 FOR UPDATE`, biz, id).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.NotFound("master katalog")
	}
	if err != nil {
		return err
	}
	if current != v {
		return errs.New(errs.CodeOptimisticLockFailure, "master berubah; muat ulang", 409)
	}
	return nil
}
func (s *Server) categoryDetail(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("id tidak valid"))
		return
	}
	var name, status string
	var v int
	var tax *string
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT name,status,version,tax_rule_id::text FROM categories WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&name, &status, &v, &tax)
	if readError(w, r, err, "kategori") {
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id.String(), "name": name, "status": status, "version": v, "tax_rule_id": tax})
}
func (s *Server) editCategory(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	restore := r.Method == "POST"
	var req struct {
		Name string `json:"name"`
	}
	if !restore {
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" || utf8.RuneCountInString(req.Name) > 200 {
			httpx.WriteError(w, r, errs.Validation("nama kategori wajib maksimal 200 karakter"))
			return
		}
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if err := lockCatalogVersion(ctx, tx, tc.BusinessID, id, "categories", v); err != nil {
			return err
		}
		var old, status string
		if err := tx.QueryRow(ctx, `SELECT name,status FROM categories WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&old, &status); err != nil {
			return err
		}
		action := "category.updated"
		if restore {
			if status != "archived" {
				return errs.Conflict("kategori sudah aktif")
			}
			req.Name = old
			action = "category.restored"
		} else if status != "active" {
			return errs.Conflict("reaktifkan kategori sebelum edit")
		}
		if _, err := tx.Exec(ctx, `UPDATE categories SET name=$3,status='active',version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Name); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: action, EntityType: "category", EntityID: id.String(), Before: map[string]string{"name": old, "status": status}, After: map[string]string{"name": req.Name, "status": "active"}})
	})
	if err != nil {
		httpx.WriteError(w, r, catalogManagementError(err))
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id.String(), "version": v + 1})
}
func (s *Server) productDetail(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("id tidak valid"))
		return
	}
	var name, status string
	var category, tax *string
	var track bool
	var version int
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT name,status,category_id::text,tax_rule_id::text,track_stock,version FROM products WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&name, &status, &category, &tax, &track, &version)
	if readError(w, r, err, "produk") {
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id.String(), "name": name, "status": status, "category_id": category, "tax_rule_id": tax, "track_stock": track, "version": version})
}
func (s *Server) editProduct(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Name     string  `json:"name"`
		Category *string `json:"category_id"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 200 {
		httpx.WriteError(w, r, errs.Validation("nama wajib maksimal 200 karakter"))
		return
	}
	var category *idkit.ID
	if req.Category != nil {
		x, err := idkit.Parse(*req.Category)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("category_id tidak valid"))
			return
		}
		category = &x
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if category != nil {
			var active bool
			err := tx.QueryRow(ctx, `SELECT status='active' FROM categories WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, *category).Scan(&active)
			if errors.Is(err, pgx.ErrNoRows) {
				return errs.NotFound("kategori")
			}
			if err != nil {
				return err
			}
			if !active {
				return errs.Conflict("kategori diarsipkan")
			}
		}
		if err := lockCatalogVersion(ctx, tx, tc.BusinessID, id, "products", v); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE products SET name=$3,category_id=$4,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Name, category); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "product.updated", EntityType: "product", EntityID: id.String(), After: map[string]any{"name": req.Name, "category_id": req.Category}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id.String(), "version": v + 1})
}
func (s *Server) productStatus(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Status != "active" && req.Status != "inactive" && req.Status != "archived" {
		httpx.WriteError(w, r, errs.Validation("status tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if err := lockCatalogVersion(ctx, tx, tc.BusinessID, id, "products", v); err != nil {
			return err
		}
		var busy bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM restaurant_order_lines l JOIN restaurant_orders o ON o.business_id=l.business_id AND o.id=l.order_id JOIN variants v ON v.business_id=l.business_id AND v.id=l.variant_id WHERE v.business_id=$1 AND v.product_id=$2 AND o.status='open' AND l.kitchen_status<>'CANCELLED')`, tc.BusinessID, id).Scan(&busy); err != nil {
			return err
		}
		if busy && req.Status != "active" {
			return errs.Conflict("produk dipakai pesanan restoran terbuka")
		}
		if _, err := tx.Exec(ctx, `UPDATE products SET status=$3,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Status); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "product.status_changed", EntityType: "product", EntityID: id.String(), After: map[string]string{"status": req.Status}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id.String(), "version": v + 1, "status": req.Status})
}
func (s *Server) editVariantIdentity(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Name string `json:"name"`
		SKU  string `json:"sku"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.SKU = strings.TrimSpace(req.SKU)
	if req.SKU == "" || utf8.RuneCountInString(req.SKU) > 100 || utf8.RuneCountInString(req.Name) > 200 {
		httpx.WriteError(w, r, errs.Validation("SKU/nama tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if err := lockCatalogVersion(ctx, tx, tc.BusinessID, id, "variants", v); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE variants SET name=$3,sku=$4,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Name, req.SKU); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "variant.identity_changed", EntityType: "variant", EntityID: id.String(), After: map[string]string{"name": req.Name, "sku": req.SKU}})
	})
	if err != nil {
		httpx.WriteError(w, r, catalogManagementError(err))
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id.String(), "version": v + 1})
}
func (s *Server) editVariantFractional(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Allow *bool `json:"allow_fractional"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Allow == nil {
		httpx.WriteError(w, r, errs.Validation("allow_fractional wajib"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if err := lockCatalogVersion(ctx, tx, tc.BusinessID, id, "variants", v); err != nil {
			return err
		}
		var pharmacy bool
		if err := tx.QueryRow(ctx, `SELECT business_type='pharmacy' FROM businesses WHERE id=$1`, tc.BusinessID).Scan(&pharmacy); err != nil {
			return err
		}
		if pharmacy && *req.Allow {
			return errs.Validation("pecahan apotek belum didukung")
		}
		if _, err := tx.Exec(ctx, `UPDATE variants SET allow_fractional=$3,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, *req.Allow); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "variant.fractional_changed", EntityType: "variant", EntityID: id.String(), After: map[string]bool{"allow_fractional": *req.Allow}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id.String(), "version": v + 1})
}
func (s *Server) listVariantBarcodes(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("id tidak valid"))
		return
	}
	var exists bool
	if err := s.pool.Underlying().QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM variants WHERE business_id=$1 AND id=$2)`, tc.BusinessID, id).Scan(&exists); readError(w, r, err, "varian") {
		return
	}
	if !exists {
		httpx.WriteError(w, r, errs.NotFound("varian"))
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT barcode,unit_id::text FROM variant_barcodes WHERE business_id=$1 AND variant_id=$2 AND barcode>$3 ORDER BY barcode LIMIT 101`, tc.BusinessID, id, r.URL.Query().Get("after"))
	if readError(w, r, err, "barcode") {
		return
	}
	defer rows.Close()
	type barcode struct {
		Code string  `json:"barcode"`
		Unit *string `json:"unit_id"`
	}
	out := []barcode{}
	for rows.Next() {
		var x barcode
		if readError(w, r, rows.Scan(&x.Code, &x.Unit), "barcode") {
			return
		}
		out = append(out, x)
	}
	if readError(w, r, rows.Err(), "barcode") {
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].Code
	}
	httpx.JSON(w, 200, map[string]any{"barcodes": out, "next_cursor": next})
}
func (s *Server) deleteVariantBarcode(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Code string `json:"barcode"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Code == "" || len(req.Code) > 200 {
		httpx.WriteError(w, r, errs.Validation("barcode tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if err := lockCatalogVersion(ctx, tx, tc.BusinessID, id, "variants", v); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM variant_barcodes WHERE business_id=$1 AND variant_id=$2 AND barcode=$3`, tc.BusinessID, id, req.Code)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errs.NotFound("barcode")
		}
		if _, err := tx.Exec(ctx, `UPDATE variants SET version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "variant.barcode_removed", EntityType: "variant", EntityID: id.String(), After: map[string]string{"barcode": req.Code}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id.String(), "version": v + 1})
}
