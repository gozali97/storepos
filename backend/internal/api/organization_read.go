package apiserver

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) getBranch(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, ok := branchOf(w, r, tc, r.PathValue("id"))
	if !ok {
		return
	}
	var x struct {
		ID       string `json:"branch_id"`
		Code     string `json:"code"`
		Name     string `json:"name"`
		Timezone string `json:"timezone"`
		Address  string `json:"address"`
		Header   string `json:"receipt_header"`
		Footer   string `json:"receipt_footer"`
		Status   string `json:"status"`
		Version  int    `json:"version"`
	}
	err := s.pool.Underlying().QueryRow(r.Context(), `SELECT id::text,code,name,timezone,COALESCE(address,''),COALESCE(receipt_header,''),COALESCE(receipt_footer,''),status,version FROM branches WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&x.ID, &x.Code, &x.Name, &x.Timezone, &x.Address, &x.Header, &x.Footer, &x.Status, &x.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, errs.NotFound("cabang"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, x)
}
func (s *Server) getRegister(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("register_id tidak valid"))
		return
	}
	var branch idkit.ID
	var x struct {
		ID      string `json:"register_id"`
		Branch  string `json:"branch_id"`
		Name    string `json:"name"`
		Status  string `json:"status"`
		Version int    `json:"version"`
	}
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT id::text,branch_id,name,status,version FROM registers WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&x.ID, &branch, &x.Name, &x.Status, &x.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, errs.NotFound("terminal"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !tc.HasBranch(branch) {
		httpx.WriteError(w, r, errs.PermissionDenied(""))
		return
	}
	x.Branch = branch.String()
	httpx.JSON(w, http.StatusOK, x)
}
