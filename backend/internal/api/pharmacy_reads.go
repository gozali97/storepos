package apiserver

import (
	"net/http"
	"strconv"
	"time"

	"github.com/storepos/storepos/internal/pharmacy"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) listBatches(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return
	}
	cursor, ok := pageCursor(w, r)
	if !ok {
		return
	}
	filter := q.Get("filter")
	if filter == "" {
		filter = "all"
	}
	if filter != "all" && filter != "near_expiry" && filter != "expired" && filter != "quarantine" {
		httpx.WriteError(w, r, errs.Validation("filter invalid"))
		return
	}
	days := 90
	if q.Get("days") != "" {
		v, err := strconv.Atoi(q.Get("days"))
		if err != nil || v < 0 || v > 3650 {
			httpx.WriteError(w, r, errs.Validation("days 0..3650"))
			return
		}
		days = v
	}
	today, err := pharmacy.BranchDate(r.Context(), s.pool.Underlying(), tc.BusinessID, branch)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT b.id::text,b.variant_id::text,v.sku,b.batch_number,b.expiry_date::text,x.status,x.qty_milli::text,
 CASE WHEN x.status='SELLABLE' AND b.expiry_date <= $3::date THEN 'EXPIRED' ELSE x.status END,
 (b.expiry_date-$3::date),last_supplier.name
 FROM pharmacy_batch_balances x JOIN pharmacy_batches b ON b.business_id=x.business_id AND b.id=x.batch_id JOIN variants v ON v.business_id=b.business_id AND v.id=b.variant_id
 LEFT JOIN LATERAL(SELECT s.name FROM pharmacy_batch_receipts r JOIN suppliers s ON s.business_id=r.business_id AND s.id=r.supplier_id WHERE r.business_id=b.business_id AND r.batch_id=b.id AND r.branch_id=x.branch_id ORDER BY r.created_at DESC,r.id DESC LIMIT 1) last_supplier ON true
 WHERE x.business_id=$1 AND x.branch_id=$2 AND b.id>$4 AND ($5='all' OR ($5='quarantine' AND x.status='QUARANTINE') OR ($5='expired' AND b.expiry_date<=$3::date AND x.qty_milli>0) OR ($5='near_expiry' AND b.expiry_date>$3::date AND b.expiry_date<=$3::date+$6 AND x.qty_milli>0)) ORDER BY b.id,x.status LIMIT 101`, tc.BusinessID, branch, today.Format("2006-01-02"), cursor, filter, days)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type batch struct {
		ID       string  `json:"batch_id"`
		Variant  string  `json:"variant_id"`
		SKU      string  `json:"sku"`
		Number   string  `json:"batch_number"`
		Expiry   string  `json:"expiry_date"`
		Stored   string  `json:"stored_status"`
		Qty      string  `json:"qty_milli"`
		Status   string  `json:"status"`
		Days     int     `json:"days_to_expiry"`
		Supplier *string `json:"last_supplier_name"`
	}
	out := []batch{}
	for rows.Next() {
		var x batch
		if err := rows.Scan(&x.ID, &x.Variant, &x.SKU, &x.Number, &x.Expiry, &x.Stored, &x.Qty, &x.Status, &x.Days, &x.Supplier); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Keep entire last batch across status buckets; explicit truncation until compound pagination is added.
	truncated := len(out) > 100
	httpx.JSON(w, 200, map[string]any{"batches": out, "truncated": truncated, "next_cursor": "", "local_date": today.Format("2006-01-02"), "expiry_warning_days": days})
}
func (s *Server) fefoPreview(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return
	}
	variant, e1 := idkit.Parse(q.Get("variant_id"))
	qty, e2 := money.ParseMilli(q.Get("qty_milli"))
	if e1 != nil || e2 != nil || qty <= 0 || qty%1000 != 0 {
		httpx.WriteError(w, r, errs.Validation("varian/qty invalid"))
		return
	}
	today, err := pharmacy.BranchDate(r.Context(), s.pool.Underlying(), tc.BusinessID, branch)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT b.id::text,b.batch_number,b.expiry_date::text,x.qty_milli,(b.expiry_date-$3::date) FROM pharmacy_batches b JOIN pharmacy_batch_balances x ON x.business_id=b.business_id AND x.batch_id=b.id WHERE b.business_id=$1 AND x.branch_id=$2 AND b.expiry_date>$3::date AND b.variant_id=$4 AND x.status='SELLABLE' AND x.qty_milli>0 ORDER BY b.expiry_date,b.id`, tc.BusinessID, branch, today.Format("2006-01-02"), variant)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	remaining := int64(qty)
	type allocation struct {
		ID     string `json:"batch_id"`
		Number string `json:"batch_number"`
		Expiry string `json:"expiry_date"`
		Qty    string `json:"qty_milli"`
		Days   int    `json:"days_to_expiry"`
	}
	out := []allocation{}
	for rows.Next() {
		var x allocation
		var available int64
		if err := rows.Scan(&x.ID, &x.Number, &x.Expiry, &available, &x.Days); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		take := available
		if take > remaining {
			take = remaining
		}
		if take > 0 {
			x.Qty = strconv.FormatInt(take, 10)
			out = append(out, x)
			remaining -= take
		}
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, 200, map[string]any{"allocations": out, "shortage_milli": strconv.FormatInt(remaining, 10), "preview_only": true, "finalization_blocked": true})
}
func (s *Server) batchMovements(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	branch, ok := branchOf(w, r, tc, r.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	batch, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("batch invalid"))
		return
	}
	cursor, ok := pageCursor(w, r)
	if !ok {
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,status,kind,qty_milli::text,balance_after_milli::text,ref_id::text,reason,created_by::text,created_at FROM pharmacy_batch_movements WHERE business_id=$1 AND branch_id=$2 AND batch_id=$3 AND id>$4 ORDER BY id LIMIT 101`, tc.BusinessID, branch, batch, cursor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type movement struct {
		ID      string    `json:"id"`
		Status  string    `json:"status"`
		Kind    string    `json:"kind"`
		Qty     string    `json:"qty_milli"`
		Balance string    `json:"balance_after_milli"`
		Ref     string    `json:"ref_id"`
		Reason  string    `json:"reason"`
		Actor   string    `json:"created_by"`
		At      time.Time `json:"created_at"`
	}
	out := []movement{}
	for rows.Next() {
		var x movement
		if err := rows.Scan(&x.ID, &x.Status, &x.Kind, &x.Qty, &x.Balance, &x.Ref, &x.Reason, &x.Actor, &x.At); err != nil {
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
	httpx.JSON(w, 200, map[string]any{"movements": out, "next_cursor": next})
}
