package reporting

import (
	"errors"
	"math/big"
	"sort"
)

type CategoryRow struct {
	CategoryID          *string `json:"category_id"`
	ProductCount        int     `json:"product_count"`
	ProductsWithoutCost int     `json:"products_without_cost"`
	Net                 *string `json:"net_sales_cents"`
	Cost                *string `json:"sold_cost_cents"`
	RestockCost         *string `json:"restock_cost_cents"`
	Profit              *string `json:"estimated_gross_profit_cents"`
}

// Do not sum quantities across products: category members can use incompatible base units.
// Aggregation uses the exact product result set used by JSON/CSV, preserving its cost visibility.
func Categories(products []ProductRow) ([]CategoryRow, error) {
	type group struct {
		row               CategoryRow
		products, missing map[string]bool
		sums              [4]*big.Int
		known             [4]bool
	}
	groups := map[string]*group{}
	for _, p := range products {
		key := ""
		if p.CategoryID != nil {
			key = *p.CategoryID
		}
		g := groups[key]
		if g == nil {
			g = &group{row: CategoryRow{CategoryID: p.CategoryID}, products: map[string]bool{}, missing: map[string]bool{}, known: [4]bool{true, true, true, true}}
			for i := range g.sums {
				g.sums[i] = new(big.Int)
			}
			groups[key] = g
		}
		g.products[p.ProductID] = true
		if p.MissingCost {
			g.missing[p.ProductID] = true
		}
		for i, v := range []*string{p.Net, p.Cost, p.RestockCost, p.Profit} {
			if v == nil {
				g.known[i] = false
				continue
			}
			n, ok := new(big.Int).SetString(*v, 10)
			if !ok {
				return nil, errors.New("angka laporan produk tidak valid")
			}
			g.sums[i].Add(g.sums[i], n)
		}
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []CategoryRow{}
	for _, k := range keys {
		g := groups[k]
		g.row.ProductCount = len(g.products)
		g.row.ProductsWithoutCost = len(g.missing)
		dest := []**string{&g.row.Net, &g.row.Cost, &g.row.RestockCost, &g.row.Profit}
		for i, d := range dest {
			if g.known[i] {
				v := g.sums[i].String()
				*d = &v
			}
		}
		out = append(out, g.row)
	}
	return out, nil
}
