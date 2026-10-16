package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/catalog"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) catalogRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/categories", s.guard(authz.PermProductView, false, s.listCategories))
	mux.Handle("POST /api/v1/categories", s.guard(authz.PermProductManage, true, s.createCategory))
	mux.Handle("GET /api/v1/catalog/search", s.guard(authz.PermProductView, false, s.searchCatalog))
	mux.Handle("POST /api/v1/products", s.guard(authz.PermProductManage, true, s.createProduct))
	mux.Handle("PATCH /api/v1/variants/{id}/price", s.guard(authz.PermProductManage, true, s.updatePrice))
}

type variantDTO struct {
	ID               string            `json:"id"`
	Name             string            `json:"name,omitempty"`
	SKU              string            `json:"sku"`
	BaseUnit         string            `json:"base_unit"`
	PriceCents       string            `json:"price_cents"` // string: ADR 0002
	PriceIncludesTax bool              `json:"price_includes_tax"`
	AllowFractional  bool              `json:"allow_fractional"`
	Attributes       map[string]string `json:"attributes"`
	Version          int               `json:"version"`
	Barcodes         []string          `json:"barcodes,omitempty"`
}

func toVariantDTO(v catalog.Variant) variantDTO {
	return variantDTO{ID: v.ID.String(), Name: v.Name, SKU: v.SKU, BaseUnit: v.BaseUnit,
		PriceCents: strconv.FormatInt(v.PriceCents, 10), PriceIncludesTax: v.PriceIncludesTax,
		Version: v.Version, Barcodes: v.Barcodes, AllowFractional: v.AllowFractional, Attributes: v.Attributes}
}

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	cs, err := catalog.NewRepo(s.pool.Underlying()).ListCategories(r.Context(), tc.BusinessID)
	if err != nil {
		httpx.WriteError(w, r, errs.Internal("gagal membaca kategori").Wrap(err))
		return
	}
	out := make([]map[string]any, 0, len(cs))
	for _, c := range cs {
		out = append(out, map[string]any{"id": c.ID.String(), "name": c.Name, "version": c.Version})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"categories": out})
}

func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.WriteError(w, r, errs.Validation("nama kategori wajib diisi"))
		return
	}
	c, err := catalog.NewRepo(s.pool.Underlying()).CreateCategory(r.Context(), tc.BusinessID, req.Name)
	if errors.Is(err, catalog.ErrCategoryTaken) {
		httpx.WriteError(w, r, errs.Conflict(err.Error()))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, errs.Internal("gagal membuat kategori").Wrap(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": c.ID.String(), "name": c.Name, "version": c.Version})
}

func (s *Server) searchCatalog(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	hits, err := catalog.NewRepo(s.pool.Underlying()).Search(r.Context(), tc.BusinessID, r.URL.Query().Get("q"), limit)
	if err != nil {
		httpx.WriteError(w, r, errs.Internal("gagal mencari katalog").Wrap(err))
		return
	}
	out := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		v := toVariantDTO(h.Variant)
		// Harga beli/HPP tidak pernah dikirim ke peran tanpa izin; pencarian kasir hanya butuh harga jual.
		out = append(out, map[string]any{"variant": v, "product_id": h.Variant.ProductID.String(), "product_name": h.ProductName, "track_stock": h.TrackStock})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

type createProductReq struct {
	Name       string  `json:"name"`
	CategoryID *string `json:"category_id"`
	TrackStock *bool   `json:"track_stock"`
	Variants   []struct {
		Name             string            `json:"name"`
		SKU              string            `json:"sku"`
		BaseUnit         string            `json:"base_unit"`
		PriceCents       string            `json:"price_cents"`
		PriceIncludesTax bool              `json:"price_includes_tax"`
		AllowFractional  bool              `json:"allow_fractional"`
		Attributes       map[string]string `json:"attributes"`
		Barcodes         []string          `json:"barcodes"`
	} `json:"variants"`
}

func (s *Server) createProduct(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req createProductReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" || len(req.Variants) == 0 {
		httpx.WriteError(w, r, errs.Validation("nama produk dan minimal satu varian wajib diisi"))
		return
	}
	p := &catalog.Product{Name: req.Name, TrackStock: req.TrackStock == nil || *req.TrackStock}
	if req.CategoryID != nil {
		id, err := idkit.Parse(*req.CategoryID)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("category_id tidak valid"))
			return
		}
		p.CategoryID = &id
	}
	nvs := make([]catalog.NewVariant, 0, len(req.Variants))
	for i, v := range req.Variants {
		price, err := money.ParseCents(v.PriceCents)
		if err != nil || price < 0 || strings.TrimSpace(v.SKU) == "" {
			httpx.WriteError(w, r, errs.Validation("varian tidak valid",
				errs.FieldError{Field: "variants[" + strconv.Itoa(i) + "]", Code: "invalid",
					Message: "SKU wajib dan harga harus bilangan sen ≥ 0 (string)"}))
			return
		}
		nvs = append(nvs, catalog.NewVariant{Name: v.Name, SKU: v.SKU, BaseUnit: v.BaseUnit,
			PriceCents: int64(price), PriceIncludesTax: v.PriceIncludesTax, Barcodes: v.Barcodes, AllowFractional: v.AllowFractional, Attributes: v.Attributes})
	}

	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if err := catalog.NewRepo(tx).CreateProduct(ctx, tc.BusinessID, p, nvs); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID,
			Action: "product.created", EntityType: "product", EntityID: p.ID.String(),
			After: map[string]any{"name": p.Name, "variants": len(nvs)}})
	})
	var bt *catalog.BarcodeTakenError
	switch {
	case errors.As(err, &bt):
		httpx.WriteError(w, r, errs.Conflict(bt.Error()))
		return
	case errors.Is(err, catalog.ErrAttributes):
		httpx.WriteError(w, r, errs.Validation(err.Error()))
		return
	case errors.Is(err, catalog.ErrSKUTaken):
		httpx.WriteError(w, r, errs.Conflict(err.Error()))
		return
	case err != nil:
		httpx.WriteError(w, r, errs.Internal("gagal membuat produk").Wrap(err))
		return
	}
	vs := make([]variantDTO, 0, len(p.Variants))
	for _, v := range p.Variants {
		vs = append(vs, toVariantDTO(v))
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": p.ID.String(), "name": p.Name, "variants": vs})
}

func (s *Server) updatePrice(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	vid, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("id varian tidak valid"))
		return
	}
	version, err := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if err != nil {
		httpx.WriteError(w, r, errs.New(errs.CodeValidation, "header If-Match (versi) wajib diisi", http.StatusPreconditionRequired))
		return
	}
	var req struct {
		PriceCents string `json:"price_cents"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	price, err := money.ParseCents(req.PriceCents)
	if err != nil || price < 0 {
		httpx.WriteError(w, r, errs.Validation("price_cents harus bilangan sen ≥ 0 (string)"))
		return
	}

	var newVersion int
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		repo := catalog.NewRepo(tx)
		old, err := repo.VariantByID(ctx, tc.BusinessID, vid)
		if err != nil {
			return err
		}
		newVersion, err = repo.UpdatePrice(ctx, tc.BusinessID, vid, version, int64(price))
		if err != nil {
			return err
		}
		// Riwayat perubahan harga masuk audit (CORE-011).
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID,
			Action: "variant.price_changed", EntityType: "variant", EntityID: vid.String(),
			Before: map[string]any{"price_cents": old.PriceCents},
			After:  map[string]any{"price_cents": int64(price)}})
	})
	switch {
	case errors.Is(err, catalog.ErrNotFound):
		httpx.WriteError(w, r, errs.NotFound("varian"))
	case errors.Is(err, catalog.ErrVersionConflict):
		httpx.WriteError(w, r, errs.New(errs.CodeOptimisticLockFailure, err.Error(), http.StatusConflict))
	case err != nil:
		httpx.WriteError(w, r, errs.Internal("gagal mengubah harga").Wrap(err))
	default:
		w.Header().Set("ETag", strconv.Itoa(newVersion))
		httpx.JSON(w, http.StatusOK, map[string]any{"id": vid.String(), "price_cents": req.PriceCents, "version": newVersion})
	}
}
