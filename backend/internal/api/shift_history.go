package apiserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/idempotency"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) shiftHistoryRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/shifts", s.guard(authz.PermShiftManageOwn, false, s.listShifts))
	mux.Handle("GET /api/v1/shifts/{id}/notes", s.guard(authz.PermShiftManageOwn, false, s.listShiftNotes))
	mux.Handle("POST /api/v1/shifts/{id}/notes", s.guard(authz.PermShiftViewAll, true, s.createShiftNote))
}

func (s *Server) listShifts(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return
	}
	var zone string
	if err := s.pool.Underlying().QueryRow(r.Context(), `SELECT timezone FROM branches WHERE business_id=$1 AND id=$2`, tc.BusinessID, branch).Scan(&zone); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = errs.NotFound("cabang")
		}
		httpx.WriteError(w, r, err)
		return
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	from, e1 := time.ParseInLocation("2006-01-02", q.Get("from"), loc)
	to, e2 := time.ParseInLocation("2006-01-02", q.Get("to"), loc)
	if e1 != nil || e2 != nil || from.After(to) || to.After(from.AddDate(1, 0, 0)) {
		httpx.WriteError(w, r, errs.Validation("periode shift wajib YYYY-MM-DD maksimal satu tahun"))
		return
	}
	status := q.Get("status")
	if status == "" {
		status = "all"
	}
	if status != "all" && status != "open" && status != "closed" {
		httpx.WriteError(w, r, errs.Validation("status harus all/open/closed"))
		return
	}
	all := authz.Check(tc.Role, authz.PermShiftViewAll) == authz.Allowed
	var register *idkit.ID
	if raw := q.Get("register_id"); raw != "" {
		id, err := idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("register_id tidak valid"))
			return
		}
		var valid bool
		if err := s.pool.Underlying().QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM registers WHERE business_id=$1 AND branch_id=$2 AND id=$3)`, tc.BusinessID, branch, id).Scan(&valid); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if !valid {
			httpx.WriteError(w, r, errs.NotFound("terminal cabang"))
			return
		}
		register = &id
	}
	var before *time.Time
	var cursor idkit.ID
	if raw := q.Get("before"); raw != "" {
		id, err := idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return
		}
		cursor = id
		var at time.Time
		err = s.pool.Underlying().QueryRow(r.Context(), `SELECT opened_at FROM shifts WHERE business_id=$1 AND branch_id=$2 AND id=$3 AND opened_at >=$4 AND opened_at<$5 AND ($6='all' OR status=$6) AND ($7 OR opened_by=$8) AND ($9::uuid IS NULL OR register_id=$9)`, tc.BusinessID, branch, id, from, to.AddDate(0, 0, 1), status, all, tc.UserID, register).Scan(&at)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				err = errs.Validation("cursor di luar cakupan/filter")
			}
			httpx.WriteError(w, r, err)
			return
		}
		before = &at
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT s.id::text,s.register_id::text,r.name,s.opened_by::text,COALESCE(NULLIF(u.display_name,''), 'Petugas'),s.opened_at,s.closed_at,s.status,
 CASE WHEN s.status='closed' THEN s.expected_cash_cents::text END,CASE WHEN s.status='closed' THEN s.counted_cash_cents::text END,CASE WHEN s.status='closed' THEN s.variance_cents::text END
 FROM shifts s JOIN registers r ON r.business_id=s.business_id AND r.id=s.register_id JOIN users u ON u.id=s.opened_by
 WHERE s.business_id=$1 AND s.branch_id=$2 AND s.opened_at >=$3 AND s.opened_at<$4 AND ($5='all' OR s.status=$5) AND ($6 OR s.opened_by=$7) AND ($8::uuid IS NULL OR s.register_id=$8)
 AND ($9::timestamptz IS NULL OR (s.opened_at,s.id)<($9,$10::uuid)) ORDER BY s.opened_at DESC,s.id DESC LIMIT 101`, tc.BusinessID, branch, from, to.AddDate(0, 0, 1), status, all, tc.UserID, register, before, cursor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type row struct {
		ID           string     `json:"shift_id"`
		Register     string     `json:"register_id"`
		RegisterName string     `json:"register_name"`
		User         string     `json:"opened_by"`
		Name         string     `json:"cashier_name"`
		Opened       time.Time  `json:"opened_at"`
		Closed       *time.Time `json:"closed_at"`
		Status       string     `json:"status"`
		Expected     *string    `json:"expected_cash_cents"`
		Counted      *string    `json:"counted_cash_cents"`
		Variance     *string    `json:"variance_cents"`
	}
	out := []row{}
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.ID, &x.Register, &x.RegisterName, &x.User, &x.Name, &x.Opened, &x.Closed, &x.Status, &x.Expected, &x.Counted, &x.Variance); err != nil {
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
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, 200, map[string]any{"shifts": out, "next_cursor": next, "timezone": zone, "scope": map[bool]string{true: "branch", false: "own"}[all]})
}

func closedShiftAccess(ctx context.Context, db database.Getter, tc tenant.Context, id idkit.ID) (idkit.ID, error) {
	var branch, owner idkit.ID
	var status string
	err := db.QueryRow(ctx, `SELECT branch_id,opened_by,status FROM shifts WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, id).Scan(&branch, &owner, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return branch, errs.NotFound("shift")
	}
	if err != nil {
		return branch, err
	}
	if !tc.HasBranch(branch) || (owner != tc.UserID && authz.Check(tc.Role, authz.PermShiftViewAll) != authz.Allowed) {
		return branch, errs.PermissionDenied("")
	}
	if status != "closed" {
		return branch, errs.Conflict("catatan koreksi hanya untuk shift tertutup")
	}
	return branch, nil
}

func (s *Server) listShiftNotes(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("shift_id tidak valid"))
		return
	}
	cursor, ok := pageCursor(w, r)
	if !ok {
		return
	}
	type note struct {
		ID   string    `json:"note_id"`
		Body string    `json:"body"`
		User string    `json:"created_by"`
		Name string    `json:"author_name"`
		At   time.Time `json:"created_at"`
	}
	out := []note{}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if _, err := closedShiftAccess(ctx, tx, tc, id); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT n.id::text,n.body,n.created_by::text,COALESCE(NULLIF(u.display_name,''),'Petugas'),n.created_at FROM shift_correction_notes n JOIN users u ON u.id=n.created_by WHERE n.business_id=$1 AND n.shift_id=$2 AND n.id>$3 ORDER BY n.id LIMIT 101`, tc.BusinessID, id, cursor)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var x note
			if err := rows.Scan(&x.ID, &x.Body, &x.User, &x.Name, &x.At); err != nil {
				return err
			}
			out = append(out, x)
		}
		return rows.Err()
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].ID
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, 200, map[string]any{"notes": out, "next_cursor": next})
}

func (s *Server) createShiftNote(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("shift_id tidak valid"))
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" || !utf8.ValidString(req.Body) || utf8.RuneCountInString(req.Body) > 1000 {
		httpx.WriteError(w, r, errs.Validation("catatan wajib maksimal 1000 karakter"))
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key wajib maksimal 128 karakter"))
		return
	}
	raw, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hash := sha256.Sum256(raw)
	var replay *idempotency.Stored
	var response []byte
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		branch, err := closedShiftAccess(ctx, tx, tc, id)
		if err != nil {
			return err
		}
		st, err := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /shifts/"+id.String()+"/notes", hash[:])
		replay = st
		if err != nil || st != nil {
			return err
		}
		nid := idkit.New()
		if _, err := tx.Exec(ctx, `INSERT INTO shift_correction_notes(id,business_id,shift_id,body,created_by) VALUES($1,$2,$3,$4,$5)`, nid, tc.BusinessID, id, req.Body, tc.UserID); err != nil {
			return err
		}
		if err := audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "shift.correction_note_added", EntityType: "shift", EntityID: id.String(), After: map[string]string{"note_id": nid.String()}}); err != nil {
			return err
		}
		response, err = json.Marshal(map[string]string{"note_id": nid.String(), "status": "recorded"})
		if err != nil {
			return err
		}
		return idempotency.Complete(ctx, tx, tc.BusinessID, key, 201, response)
	})
	if err != nil {
		if errors.Is(err, idempotency.ErrConflict) {
			err = mapSalesError(err)
		}
		httpx.WriteError(w, r, err)
		return
	}
	if replay != nil {
		response = replay.Body
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(201)
	_, _ = w.Write(response)
}
