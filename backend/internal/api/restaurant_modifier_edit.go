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

func (s *Server) editModifierGroup(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Name     string `json:"name"`
		Min      int    `json:"min_choices"`
		Max      int    `json:"max_choices"`
		Archived bool   `json:"archived"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 100 || req.Min < 0 || req.Max < req.Min || req.Max < 1 || req.Max > 100 {
		httpx.WriteError(w, r, errs.Validation("nama/min/max tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current, min, max int
		var name string
		var archived bool
		err := tx.QueryRow(ctx, `SELECT version,name,min_choices,max_choices,archived FROM restaurant_modifier_groups WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&current, &name, &min, &max, &archived)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("grup modifier")
		}
		if err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "grup modifier berubah", http.StatusConflict)
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_modifier_groups SET name=$3,min_choices=$4,max_choices=$5,archived=$6,version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Name, req.Min, req.Max, req.Archived); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "modifier.group_updated", EntityType: "modifier_group", EntityID: id.String(), Before: map[string]any{"name": name, "min_choices": min, "max_choices": max, "archived": archived}, After: map[string]any{"name": req.Name, "min_choices": req.Min, "max_choices": req.Max, "archived": req.Archived}})
	})
	if err != nil {
		httpx.WriteError(w, r, restaurantError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"group_id": id.String(), "version": v + 1})
}
func (s *Server) editModifierOption(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	group, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	option, err := idkit.Parse(r.PathValue("option"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("option_id tidak valid"))
		return
	}
	var req struct {
		Name   string `json:"name"`
		Price  string `json:"price_delta_cents"`
		Active bool   `json:"active"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	price, err := money.ParseCents(req.Price)
	if err != nil || price < 0 || price > 10000000000 || req.Name == "" || utf8.RuneCountInString(req.Name) > 100 {
		httpx.WriteError(w, r, errs.Validation("nama/harga tidak valid"))
		return
	}
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
		var name string
		var oldPrice int64
		var active bool
		err = tx.QueryRow(ctx, `SELECT name,price_delta_cents,active FROM restaurant_modifier_options WHERE business_id=$1 AND group_id=$2 AND id=$3 FOR UPDATE`, tc.BusinessID, group, option).Scan(&name, &oldPrice, &active)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("opsi modifier")
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_modifier_options SET name=$4,price_delta_cents=$5,active=$6 WHERE business_id=$1 AND group_id=$2 AND id=$3`, tc.BusinessID, group, option, req.Name, int64(price), req.Active); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_modifier_groups SET version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, group); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "modifier.option_updated", EntityType: "modifier_option", EntityID: option.String(), Before: map[string]any{"name": name, "price_delta_cents": oldPrice, "active": active}, After: map[string]any{"name": req.Name, "price_delta_cents": req.Price, "active": req.Active}})
	})
	if err != nil {
		httpx.WriteError(w, r, restaurantError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"option_id": option.String(), "group_version": v + 1})
}
