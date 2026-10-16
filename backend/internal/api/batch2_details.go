package apiserver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/catalog"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) batch2Routes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/settings/tax", s.guard(authz.PermBusinessManage, false, s.readTax))
	mux.Handle("GET /api/v1/categories/archived", s.guard(authz.PermProductManage, false, s.archivedCategories))
	mux.Handle("GET /api/v1/variants/{id}", s.guard(authz.PermProductView, false, s.variantDetail))
	mux.Handle("GET /api/v1/purchase-units/{id}", s.guard(authz.PermProductView, false, s.purchaseUnitDetail))
	mux.Handle("GET /api/v1/branches", s.guard(authz.PermBranchView, false, s.branchList))
	authed := func(h http.HandlerFunc) http.Handler { return CSRFMiddleware(s.secure)(RequireAuth(s)(h)) }
	mux.Handle("GET /api/v1/me/sessions", authed(s.sessionList))
	mux.Handle("POST /api/v1/me/sessions/revoke-others", authed(s.revokeOtherSessions))
	mux.Handle("GET /api/v1/report-exports", CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(http.HandlerFunc(s.exportHistory)))))
	mux.Handle("GET /api/v1/stock/counts", s.guard(authz.PermStockCountInput, false, s.countHistory))
	mux.Handle("GET /api/v1/expenses/{id}", s.guard(authz.PermExpenseManage, false, s.expenseDetail))
}
func pageCursor(w http.ResponseWriter, r *http.Request) (idkit.ID, bool) {
	var id idkit.ID
	if raw := r.URL.Query().Get("after"); raw != "" {
		var err error
		id, err = idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return id, false
		}
	}
	return id, true
}
func readError(w http.ResponseWriter, r *http.Request, err error, entity string) bool {
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, errs.NotFound(entity))
		return true
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return true
	}
	return false
}
func (s *Server) readTax(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var name, mode string
	var rate int
	var id string
	err := s.pool.Underlying().QueryRow(r.Context(), `SELECT id::text,name,rate_bps,mode FROM tax_rules WHERE business_id=$1 AND active`, tc.BusinessID).Scan(&id, &name, &rate, &mode)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.JSON(w, http.StatusOK, map[string]any{"rule": nil})
		return
	}
	if readError(w, r, err, "pajak") {
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"rule": map[string]any{"id": id, "name": name, "rate_bps": rate, "mode": mode}})
}
func (s *Server) archivedCategories(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	cursor, ok := pageCursor(w, r)
	if !ok {
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,name,version FROM categories WHERE business_id=$1 AND status='archived' AND id>$2 ORDER BY id LIMIT 101`, tc.BusinessID, cursor)
	if readError(w, r, err, "kategori") {
		return
	}
	defer rows.Close()
	type category struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Version int    `json:"version"`
	}
	out := []category{}
	for rows.Next() {
		var x category
		if err := rows.Scan(&x.ID, &x.Name, &x.Version); readError(w, r, err, "kategori") {
			return
		}
		out = append(out, x)
	}
	if readError(w, r, rows.Err(), "kategori") {
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].ID
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"categories": out, "next_cursor": next})
}
func (s *Server) variantDetail(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("id tidak valid"))
		return
	}
	v, err := catalog.NewRepo(s.pool.Underlying()).VariantByID(r.Context(), tc.BusinessID, id)
	if errors.Is(err, catalog.ErrNotFound) {
		httpx.WriteError(w, r, errs.NotFound("varian"))
		return
	}
	if readError(w, r, err, "varian") {
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"variant": toVariantDTO(*v), "status": v.Status, "product_id": v.ProductID.String()})
}
func (s *Server) purchaseUnitDetail(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("id tidak valid"))
		return
	}
	var variant, name, factor string
	var price *string
	var archived, used bool
	var version int
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT variant_id::text,name,factor_milli::text,sale_price_cents::text,archived,used_at IS NOT NULL,version FROM purchase_units WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&variant, &name, &factor, &price, &archived, &used, &version)
	if readError(w, r, err, "satuan") {
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"unit_id": id.String(), "variant_id": variant, "name": name, "factor_milli": factor, "sale_price_cents": price, "archived": archived, "factor_locked": used, "version": version})
}
func (s *Server) branchList(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	cursor, ok := pageCursor(w, r)
	if !ok {
		return
	}
	assigned := []string{}
	for _, b := range tc.BranchIDs {
		assigned = append(assigned, b.String())
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,code,name,timezone,status,version FROM branches WHERE business_id=$1 AND id>$2 AND ($3 OR id=ANY($4::uuid[])) ORDER BY id LIMIT 101`, tc.BusinessID, cursor, tc.Role == tenant.RoleOwner, assigned)
	if readError(w, r, err, "cabang") {
		return
	}
	defer rows.Close()
	type branch struct {
		ID       string `json:"branch_id"`
		Code     string `json:"code"`
		Name     string `json:"name"`
		Timezone string `json:"timezone"`
		Status   string `json:"status"`
		Version  int    `json:"version"`
	}
	out := []branch{}
	for rows.Next() {
		var x branch
		if readError(w, r, rows.Scan(&x.ID, &x.Code, &x.Name, &x.Timezone, &x.Status, &x.Version), "cabang") {
			return
		}
		out = append(out, x)
	}
	if readError(w, r, rows.Err(), "cabang") {
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].ID
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"branches": out, "next_cursor": next})
}
func (s *Server) sessionList(w http.ResponseWriter, r *http.Request) {
	info := MustSession(r.Context())
	user, err := idkit.Parse(info.UserID)
	if err != nil {
		httpx.WriteError(w, r, errs.Unauthenticated(""))
		return
	}
	cursor, ok := pageCursor(w, r)
	if !ok {
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,created_at,last_seen_at,expires_at,COALESCE(user_agent,''),COALESCE(host(ip),'') FROM sessions WHERE user_id=$1 AND revoked_at IS NULL AND expires_at>now() AND id>$2 ORDER BY id LIMIT 101`, user, cursor)
	if readError(w, r, err, "sesi") {
		return
	}
	defer rows.Close()
	type session struct {
		ID      string    `json:"session_id"`
		Created time.Time `json:"created_at"`
		Seen    time.Time `json:"last_seen_at"`
		Expires time.Time `json:"expires_at"`
		Agent   string    `json:"user_agent"`
		IP      string    `json:"ip"`
		Current bool      `json:"current"`
	}
	out := []session{}
	for rows.Next() {
		var x session
		if readError(w, r, rows.Scan(&x.ID, &x.Created, &x.Seen, &x.Expires, &x.Agent, &x.IP), "sesi") {
			return
		}
		x.Current = x.ID == info.SessionID
		out = append(out, x)
	}
	if readError(w, r, rows.Err(), "sesi") {
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].ID
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"sessions": out, "next_cursor": next})
}
func (s *Server) revokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	info := MustSession(r.Context())
	user, e1 := idkit.Parse(info.UserID)
	keep, e2 := idkit.Parse(info.SessionID)
	if e1 != nil || e2 != nil {
		httpx.WriteError(w, r, errs.Unauthenticated(""))
		return
	}
	var n int64
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE user_id=$1 AND id<>$2 AND revoked_at IS NULL`, user, keep)
		if err != nil {
			return err
		}
		n = tag.RowsAffected()
		return audit.Write(ctx, tx, audit.Entry{ActorUserID: user, Action: "auth.other_sessions_revoked", EntityType: "user", EntityID: user.String(), After: map[string]int64{"sessions": n}})
	})
	if readError(w, r, err, "sesi") {
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int64{"revoked": n})
}
func (s *Server) exportHistory(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	cursor, ok := pageCursor(w, r)
	if !ok {
		return
	}
	branches := []string{}
	for _, b := range tc.BranchIDs {
		branches = append(branches, b.String())
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,report_kind,status,row_count,created_at,expires_at FROM report_exports WHERE business_id=$1 AND requested_by=$2 AND id>$3 AND ($4 OR branch_id=ANY($5::uuid[])) ORDER BY id LIMIT 101`, tc.BusinessID, tc.UserID, cursor, tc.Role == tenant.RoleOwner, branches)
	if readError(w, r, err, "ekspor") {
		return
	}
	defer rows.Close()
	type export struct {
		ID      string    `json:"export_id"`
		Kind    string    `json:"report_kind"`
		Status  string    `json:"status"`
		Rows    *int64    `json:"row_count"`
		Created time.Time `json:"created_at"`
		Expires time.Time `json:"expires_at"`
	}
	out := []export{}
	for rows.Next() {
		var x export
		if readError(w, r, rows.Scan(&x.ID, &x.Kind, &x.Status, &x.Rows, &x.Created, &x.Expires), "ekspor") {
			return
		}
		out = append(out, x)
	}
	if readError(w, r, rows.Err(), "ekspor") {
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].ID
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"exports": out, "next_cursor": next})
}
func (s *Server) countHistory(w http.ResponseWriter, r *http.Request) {
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
	status := q.Get("status")
	if status != "" && status != "draft" && status != "posted" {
		httpx.WriteError(w, r, errs.Validation("status tidak valid"))
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,status,version,created_at,posted_at FROM stock_counts WHERE business_id=$1 AND branch_id=$2 AND id>$3 AND ($4='' OR status=$4) ORDER BY id LIMIT 101`, tc.BusinessID, branch, cursor, status)
	if readError(w, r, err, "opname") {
		return
	}
	defer rows.Close()
	type count struct {
		ID      string     `json:"count_id"`
		Status  string     `json:"status"`
		Version int        `json:"version"`
		Created time.Time  `json:"created_at"`
		Posted  *time.Time `json:"posted_at"`
	}
	out := []count{}
	for rows.Next() {
		var x count
		if readError(w, r, rows.Scan(&x.ID, &x.Status, &x.Version, &x.Created, &x.Posted), "opname") {
			return
		}
		out = append(out, x)
	}
	if readError(w, r, rows.Err(), "opname") {
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].ID
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"counts": out, "next_cursor": next})
}
func (s *Server) expenseDetail(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("id tidak valid"))
		return
	}
	var branch idkit.ID
	var date, category, amount, method, note string
	var shift, approved *string
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT branch_id,expense_date::text,category,amount_cents::text,method,note,shift_id::text,approved_by::text FROM expenses WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&branch, &date, &category, &amount, &method, &note, &shift, &approved)
	if readError(w, r, err, "pengeluaran") {
		return
	}
	if !tc.HasBranch(branch) {
		httpx.WriteError(w, r, errs.PermissionDenied(""))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"expense_id": id.String(), "branch_id": branch.String(), "expense_date": date, "category": category, "amount_cents": amount, "method": method, "note": note, "shift_id": shift, "approved_by": approved})
}
