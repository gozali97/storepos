package apiserver

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

type receiptListRow struct {
	ID           string     `json:"receipt_id"`
	Date         string     `json:"received_date"`
	Invoice      string     `json:"invoice_number"`
	SupplierID   *string    `json:"supplier_id"`
	SupplierName *string    `json:"supplier_name"`
	Status       string     `json:"status"`
	Version      int        `json:"version"`
	Created      time.Time  `json:"created_at"`
	Posted       *time.Time `json:"posted_at"`
}

func (s *Server) listGoodsReceipts(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return
	}
	from, e1 := time.Parse("2006-01-02", q.Get("from"))
	to, e2 := time.Parse("2006-01-02", q.Get("to"))
	if e1 != nil || e2 != nil || from.After(to) || to.Sub(from) > 366*24*time.Hour {
		httpx.WriteError(w, r, errs.Validation("periode tanggal dokumen wajib YYYY-MM-DD, maksimal 366 hari"))
		return
	}
	status := q.Get("status")
	if status != "" && status != "draft" && status != "posted" {
		httpx.WriteError(w, r, errs.Validation("status harus draft/posted"))
		return
	}
	invoice := strings.TrimSpace(q.Get("invoice_number"))
	if len(invoice) > 200 {
		httpx.WriteError(w, r, errs.Validation("faktur maksimal 200 byte"))
		return
	}
	var supplier *idkit.ID
	if raw := q.Get("supplier_id"); raw != "" {
		id, err := idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("supplier_id tidak valid"))
			return
		}
		supplier = &id
	}
	limit := 100
	if raw := q.Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > 100 {
			httpx.WriteError(w, r, errs.Validation("limit harus 1..100"))
			return
		}
		limit = v
	}
	var cursor idkit.ID
	var before *time.Time
	if raw := q.Get("before"); raw != "" {
		id, err := idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return
		}
		cursor = id
		var at time.Time
		err = s.pool.Underlying().QueryRow(r.Context(), `SELECT created_at FROM goods_receipts WHERE business_id=$1 AND branch_id=$2 AND id=$3 AND received_date BETWEEN $4 AND $5 AND ($6='' OR status=$6) AND ($7='' OR invoice_number=$7) AND ($8::uuid IS NULL OR supplier_id=$8)`, tc.BusinessID, branch, id, from, to, status, invoice, supplier).Scan(&at)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, errs.Validation("cursor di luar cakupan/filter"))
			return
		}
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		before = &at
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT g.id::text,g.received_date::text,g.invoice_number,g.supplier_id::text,s.name,g.status,g.version,g.created_at,g.posted_at
 FROM goods_receipts g LEFT JOIN suppliers s ON s.business_id=g.business_id AND s.id=g.supplier_id
 WHERE g.business_id=$1 AND g.branch_id=$2 AND g.received_date BETWEEN $3 AND $4
 AND ($5='' OR g.status=$5) AND ($6='' OR g.invoice_number=$6) AND ($7::uuid IS NULL OR g.supplier_id=$7)
 AND ($8::timestamptz IS NULL OR (g.created_at,g.id)<($8,$9::uuid)) ORDER BY g.created_at DESC,g.id DESC LIMIT $10`, tc.BusinessID, branch, from, to, status, invoice, supplier, before, cursor, limit+1)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	out := []receiptListRow{}
	for rows.Next() {
		var x receiptListRow
		if err := rows.Scan(&x.ID, &x.Date, &x.Invoice, &x.SupplierID, &x.SupplierName, &x.Status, &x.Version, &x.Created, &x.Posted); err != nil {
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
	if len(out) > limit {
		out = out[:limit]
		next = out[limit-1].ID
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"receipts": out, "next_cursor": next, "date_basis": "received_date"})
}
