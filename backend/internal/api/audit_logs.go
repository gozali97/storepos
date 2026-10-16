package apiserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/platform/authz"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

type auditLogRow struct {
	ID         string          `json:"id"`
	Branch     *string         `json:"branch_id"`
	Register   *string         `json:"register_id"`
	Actor      *string         `json:"actor_user_id"`
	Approver   *string         `json:"approved_by_user_id"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   *string         `json:"entity_id"`
	Before     json.RawMessage `json:"before"`
	After      json.RawMessage `json:"after"`
	Reason     *string         `json:"reason"`
	IP         *string         `json:"ip"`
	UserAgent  *string         `json:"user_agent"`
	At         time.Time       `json:"occurred_at"`
}

func (s *Server) auditRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/audit-logs", s.guard(authz.PermAuditView, false, s.listAuditLogs))
}

// UTC boundaries are explicit RFC3339 instants; branch-less tenant events are Owner-only.
// Global platform events (business_id NULL) are never exposed by this tenant endpoint.
func (s *Server) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	from, e1 := time.Parse(time.RFC3339, q.Get("from"))
	until, e2 := time.Parse(time.RFC3339, q.Get("until"))
	if e1 != nil || e2 != nil || !from.Before(until) || until.Sub(from) > 366*24*time.Hour {
		httpx.WriteError(w, r, errs.Validation("from/until RFC3339 wajib, interval maksimal 366 hari, until eksklusif"))
		return
	}
	limit := 100
	if q.Get("limit") != "" {
		v, err := strconv.Atoi(q.Get("limit"))
		if err != nil || v < 1 || v > 100 {
			httpx.WriteError(w, r, errs.Validation("limit harus 1..100"))
			return
		}
		limit = v
	}
	var branch, actor *idkit.ID
	if q.Get("branch_id") != "" {
		id, ok := branchOf(w, r, tc, q.Get("branch_id"))
		if !ok {
			return
		}
		branch = &id
	}
	if q.Get("actor_user_id") != "" {
		id, err := idkit.Parse(q.Get("actor_user_id"))
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("actor_user_id tidak valid"))
			return
		}
		actor = &id
	}
	if len(q.Get("action")) > 100 || len(q.Get("entity_type")) > 100 {
		httpx.WriteError(w, r, errs.Validation("filter action/entity_type maksimal 100 byte"))
		return
	}
	var cursor idkit.ID
	var beforeTime *time.Time
	// Use IDs as strings for pgx UUID array encoding.
	branches := make([]string, 0, len(tc.BranchIDs))
	for _, id := range tc.BranchIDs {
		branches = append(branches, id.String())
	}
	owner := tc.Role == tenant.RoleOwner
	if raw := q.Get("before"); raw != "" {
		id, err := idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return
		}
		cursor = id
		var at time.Time
		err = s.pool.Underlying().QueryRow(r.Context(), `SELECT occurred_at FROM audit_logs WHERE business_id=$1 AND id=$2
   AND ($3 OR branch_id=ANY($4::uuid[])) AND ($5::uuid IS NULL OR branch_id=$5)
   AND occurred_at >= $6 AND occurred_at < $7 AND ($8='' OR action=$8) AND ($9='' OR entity_type=$9)
   AND ($10::uuid IS NULL OR actor_user_id=$10)`, tc.BusinessID, id, owner, branches, branch, from, until, q.Get("action"), q.Get("entity_type"), actor).Scan(&at)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, errs.Validation("cursor tidak ditemukan dalam cakupan/filter ini"))
			return
		}
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		beforeTime = &at
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,branch_id::text,register_id::text,actor_user_id::text,approved_by_user_id::text,
 action,entity_type,entity_id,before_jsonb,after_jsonb,reason,host(ip),user_agent,occurred_at
 FROM audit_logs WHERE business_id=$1 AND ($2 OR branch_id=ANY($3::uuid[]))
 AND ($4::uuid IS NULL OR branch_id=$4) AND occurred_at >= $5 AND occurred_at < $6
 AND ($7='' OR action=$7) AND ($8='' OR entity_type=$8) AND ($9::uuid IS NULL OR actor_user_id=$9)
 AND ($10::timestamptz IS NULL OR (occurred_at,id)<($10,$11::uuid))
 ORDER BY occurred_at DESC,id DESC LIMIT $12`, tc.BusinessID, owner, branches, branch, from, until, q.Get("action"), q.Get("entity_type"), actor, beforeTime, cursor, limit+1)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	out := []auditLogRow{}
	for rows.Next() {
		var x auditLogRow
		if err := rows.Scan(&x.ID, &x.Branch, &x.Register, &x.Actor, &x.Approver, &x.Action, &x.EntityType, &x.EntityID, &x.Before, &x.After, &x.Reason, &x.IP, &x.UserAgent, &x.At); err != nil {
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
	httpx.JSON(w, http.StatusOK, map[string]any{"events": out, "next_cursor": next})
}
