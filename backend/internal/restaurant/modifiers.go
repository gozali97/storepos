package restaurant

import (
	"context"
	"errors"
	"math"
	"sort"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

var ErrModifierSelection = errors.New("pilihan modifier tidak valid; periksa grup wajib dan minimum/maksimum")

type ModifierSnapshot struct {
	OptionID  string `json:"option_id"`
	GroupID   string `json:"group_id"`
	GroupName string `json:"group_name"`
	Name      string `json:"name"`
	Price     int64  `json:"price_delta_cents,string"`
}

func SelectModifiers(ctx context.Context, db database.Getter, biz, product idkit.ID, selected []idkit.ID) ([]ModifierSnapshot, int64, error) {
	if len(selected) > 300 {
		return nil, 0, ErrModifierSelection
	}
	wanted := map[string]bool{}
	for _, id := range selected {
		if wanted[id.String()] {
			return nil, 0, ErrModifierSelection
		}
		wanted[id.String()] = true
	}
	rows, err := db.Query(ctx, `SELECT id,name,min_choices,max_choices FROM restaurant_modifier_groups WHERE business_id=$1 AND product_id=$2 AND NOT archived ORDER BY id FOR SHARE`, biz, product)
	if err != nil {
		return nil, 0, err
	}
	type group struct {
		id       idkit.ID
		name     string
		min, max int
	}
	var groups []group
	for rows.Next() {
		var g group
		if err := rows.Scan(&g.id, &g.name, &g.min, &g.max); err != nil {
			rows.Close()
			return nil, 0, err
		}
		groups = append(groups, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	out := []ModifierSnapshot{}
	var delta int64
	for _, g := range groups {
		rows, err := db.Query(ctx, `SELECT id::text,name,price_delta_cents FROM restaurant_modifier_options WHERE business_id=$1 AND group_id=$2 AND active ORDER BY id FOR SHARE`, biz, g.id)
		if err != nil {
			return nil, 0, err
		}
		n := 0
		for rows.Next() {
			var option, name string
			var price int64
			if err := rows.Scan(&option, &name, &price); err != nil {
				rows.Close()
				return nil, 0, err
			}
			if wanted[option] {
				if price < 0 || delta > math.MaxInt64-price {
					rows.Close()
					return nil, 0, ErrModifierSelection
				}
				delta += price
				n++
				delete(wanted, option)
				out = append(out, ModifierSnapshot{option, g.id.String(), g.name, name, price})
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, 0, err
		}
		if n < g.min || n > g.max {
			return nil, 0, ErrModifierSelection
		}
	}
	if len(wanted) > 0 {
		return nil, 0, ErrModifierSelection
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OptionID < out[j].OptionID })
	return out, delta, nil
}
