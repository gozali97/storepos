package apiserver

import (
	"net/http"

	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

// pharmacyReorder memakai stok jual batch, tidak merekonstruksi dari inventory_balances inti.
func (s *Server) pharmacyReorder(w http.ResponseWriter, r *http.Request, tc tenant.Context, branch, cursor idkit.ID) {
	rows, err := s.pool.Underlying().Query(r.Context(), `WITH local_day AS (
 SELECT (clock_timestamp() AT TIME ZONE timezone)::date AS today FROM branches WHERE business_id=$1 AND id=$2
 ), available AS (
 SELECT b.variant_id,SUM(x.qty_milli) AS qty FROM pharmacy_batch_balances x
 JOIN pharmacy_batches b ON b.business_id=x.business_id AND b.id=x.batch_id CROSS JOIN local_day d
 WHERE x.business_id=$1 AND x.branch_id=$2 AND x.status='SELLABLE' AND b.expiry_date>d.today GROUP BY b.variant_id
 ) SELECT v.id::text,v.sku,p.name,v.base_unit,COALESCE(a.qty,0)::text,m.minimum_qty_milli::text,m.version,last_supplier.id::text,last_supplier.name,'0',COALESCE(a.qty,0)::text
 FROM stock_minimums m JOIN variants v ON v.business_id=m.business_id AND v.id=m.variant_id
 JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id LEFT JOIN available a ON a.variant_id=v.id
 LEFT JOIN LATERAL (
 SELECT s.id,s.name FROM pharmacy_batch_receipts r JOIN pharmacy_batches b ON b.business_id=r.business_id AND b.id=r.batch_id
 JOIN suppliers s ON s.business_id=r.business_id AND s.id=r.supplier_id
 WHERE r.business_id=m.business_id AND r.branch_id=m.branch_id AND b.variant_id=m.variant_id ORDER BY r.created_at DESC,r.id DESC LIMIT 1
 ) last_supplier ON true
 WHERE m.business_id=$1 AND m.branch_id=$2 AND v.id>$3 AND v.status='active' AND p.status='active' AND p.track_stock
 AND COALESCE(a.qty,0)<=m.minimum_qty_milli ORDER BY v.id LIMIT 101`, tc.BusinessID, branch, cursor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	out := []reorderItem{}
	for rows.Next() {
		var x reorderItem
		if err := rows.Scan(&x.VariantID, &x.SKU, &x.Name, &x.Unit, &x.Balance, &x.Minimum, &x.Version, &x.SupplierID, &x.SupplierName, &x.Reserved, &x.Available); err != nil {
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
		next = out[99].VariantID
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, 200, map[string]any{"items": out, "next_cursor": next, "balance_basis": "sellable_nonexpired_batches"})
}
