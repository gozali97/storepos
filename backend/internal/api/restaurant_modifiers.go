package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) modifierRoutes(mux *http.ServeMux) {
	mux.Handle("PUT /api/v1/restaurant/modifier-groups/{id}", s.restaurantGuard(true, s.editModifierGroup))
	mux.Handle("PUT /api/v1/restaurant/modifier-groups/{id}/options/{option}", s.restaurantGuard(true, s.editModifierOption))
	mux.Handle("POST /api/v1/restaurant/modifier-groups", s.restaurantGuard(true, s.createModifierGroup))
	mux.Handle("POST /api/v1/restaurant/modifier-groups/{id}/options", s.restaurantGuard(true, s.createModifierOption))
	mux.Handle("GET /api/v1/restaurant/products/{id}/modifiers", s.orderGuard(s.getProductModifiers))
}
func (s *Server) createModifierGroup(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		Product string `json:"product_id"`
		Name    string `json:"name"`
		Min     int    `json:"min_choices"`
		Max     int    `json:"max_choices"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	product, err := idkit.Parse(req.Product)
	req.Name = strings.TrimSpace(req.Name)
	if err != nil || req.Name == "" || utf8.RuneCountInString(req.Name) > 100 || req.Min < 0 || req.Max < req.Min || req.Max < 1 || req.Max > 100 {
		httpx.WriteError(w, r, errs.Validation("produk/nama/min/max tidak valid"))
		return
	}
	id := idkit.New()
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var active bool
		err := tx.QueryRow(ctx, `SELECT status='active' FROM products WHERE business_id=$1 AND id=$2`, tc.BusinessID, product).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("produk")
		}
		if err != nil {
			return err
		}
		if !active {
			return errs.Conflict("produk nonaktif")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO restaurant_modifier_groups(id,business_id,product_id,name,min_choices,max_choices) VALUES($1,$2,$3,$4,$5,$6)`, id, tc.BusinessID, product, req.Name, req.Min, req.Max); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "modifier.group_created", EntityType: "modifier_group", EntityID: id.String(), After: map[string]int{"min_choices": req.Min, "max_choices": req.Max}})
	})
	if err != nil {
		httpx.WriteError(w, r, restaurantError(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"group_id": id.String(), "version": 1})
}
func (s *Server) createModifierOption(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	group, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Name  string `json:"name"`
		Price string `json:"price_delta_cents"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	price, err := money.ParseCents(req.Price)
	if err != nil || price < 0 || price > 10000000000 || req.Name == "" || utf8.RuneCountInString(req.Name) > 100 {
		httpx.WriteError(w, r, errs.Validation("nama/harga tambahan tidak valid"))
		return
	}
	id := idkit.New()
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current int
		var archived bool
		err := tx.QueryRow(ctx, `SELECT version,archived FROM restaurant_modifier_groups WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, group).Scan(&current, &archived)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("grup modifier")
		}
		if err != nil {
			return err
		}
		if current != v || archived {
			return errs.New(errs.CodeOptimisticLockFailure, "grup berubah atau diarsipkan", http.StatusConflict)
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM restaurant_modifier_options WHERE business_id=$1 AND group_id=$2`, tc.BusinessID, group).Scan(&n); err != nil {
			return err
		}
		if n >= 100 {
			return errs.Conflict("maksimal 100 opsi per grup")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO restaurant_modifier_options(id,business_id,group_id,name,price_delta_cents) VALUES($1,$2,$3,$4,$5)`, id, tc.BusinessID, group, req.Name, int64(price)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_modifier_groups SET version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, group); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "modifier.option_created", EntityType: "modifier_option", EntityID: id.String(), After: map[string]string{"price_delta_cents": req.Price}})
	})
	if err != nil {
		httpx.WriteError(w, r, restaurantError(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"option_id": id.String(), "group_version": v + 1})
}
func (s *Server) getProductModifiers(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	product, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("product_id tidak valid"))
		return
	}
	var exists bool
	if err := s.pool.Underlying().QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM products WHERE business_id=$1 AND id=$2)`, tc.BusinessID, product).Scan(&exists); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !exists {
		httpx.WriteError(w, r, errs.NotFound("produk"))
		return
	}
	include, ok := includeArchivedRestaurant(w, r)
	if !ok {
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT g.id::text,g.name,g.min_choices,g.max_choices,g.version,g.archived,o.id::text,o.name,o.price_delta_cents::text,o.active FROM restaurant_modifier_groups g LEFT JOIN restaurant_modifier_options o ON o.business_id=g.business_id AND o.group_id=g.id AND ($3 OR o.active) WHERE g.business_id=$1 AND g.product_id=$2 AND ($3 OR NOT g.archived) ORDER BY g.id,o.id`, tc.BusinessID, product, include)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type option struct {
		ID     string `json:"option_id"`
		Name   string `json:"name"`
		Price  string `json:"price_delta_cents"`
		Active bool   `json:"active"`
	}
	type group struct {
		ID       string   `json:"group_id"`
		Name     string   `json:"name"`
		Min      int      `json:"min_choices"`
		Max      int      `json:"max_choices"`
		Version  int      `json:"version"`
		Archived bool     `json:"archived"`
		Options  []option `json:"options"`
	}
	out := []group{}
	index := map[string]int{}
	for rows.Next() {
		var g group
		var id, name, price *string
		var active *bool
		if err := rows.Scan(&g.ID, &g.Name, &g.Min, &g.Max, &g.Version, &g.Archived, &id, &name, &price, &active); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		i, ok := index[g.ID]
		if !ok {
			i = len(out)
			index[g.ID] = i
			g.Options = []option{}
			out = append(out, g)
		}
		if id != nil {
			out[i].Options = append(out[i].Options, option{*id, *name, *price, *active})
		}
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"groups": out})
}
