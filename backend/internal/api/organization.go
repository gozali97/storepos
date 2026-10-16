package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/org"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) organizationRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/branches/{id}", s.guard(authz.PermBranchView, false, s.getBranch))
	mux.Handle("GET /api/v1/registers/{id}", s.guard(authz.PermBranchView, false, s.getRegister))
	mux.Handle("POST /api/v1/branches", s.guard(authz.PermBranchManage, true, s.createBranch))
	mux.Handle("PUT /api/v1/branches/{id}", s.guard(authz.PermBranchManage, true, s.updateBranch))
	mux.Handle("POST /api/v1/branches/{id}/registers", s.guard(authz.PermBranchManage, true, s.createRegister))
	mux.Handle("PATCH /api/v1/registers/{id}/status", s.guard(authz.PermBranchManage, true, s.registerStatus))
}

type branchRequest struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
	Address  string `json:"address"`
	Header   string `json:"receipt_header"`
	Footer   string `json:"receipt_footer"`
}

func validateBranch(b *branchRequest) error {
	b.Code = strings.TrimSpace(b.Code)
	b.Name = strings.TrimSpace(b.Name)
	if b.Timezone == "" {
		b.Timezone = "Asia/Jakarta"
	}
	if b.Name == "" || len(b.Code) == 0 || len(b.Code) > 20 || utf8.RuneCountInString(b.Name) > 200 || utf8.RuneCountInString(b.Address) > 1000 || utf8.RuneCountInString(b.Header) > 1000 || utf8.RuneCountInString(b.Footer) > 1000 {
		return errs.Validation("nama/kode/alamat/struk melebihi batas atau kosong")
	}
	for _, c := range b.Code {
		if !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != '-' {
			return errs.Validation("kode cabang harus huruf besar/angka/tanda hubung")
		}
	}
	if _, err := time.LoadLocation(b.Timezone); err != nil {
		return errs.Validation("zona waktu IANA tidak valid")
	}
	return nil
}
func (s *Server) createBranch(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req branchRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateBranch(&req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	b := &org.Branch{BusinessID: tc.BusinessID, Code: req.Code, Name: req.Name, Timezone: req.Timezone, Address: req.Address, ReceiptHeader: req.Header, ReceiptFooter: req.Footer}
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if err := org.NewRepo(tx).CreateBranch(ctx, b); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &b.ID, ActorUserID: tc.UserID, Action: "branch.created", EntityType: "branch", EntityID: b.ID.String(), After: map[string]string{"code": req.Code, "timezone": req.Timezone}})
	})
	if errors.Is(err, org.ErrCodeTaken) {
		httpx.WriteError(w, r, errs.Conflict(err.Error()))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"branch_id": b.ID.String(), "version": 1})
}
func (s *Server) updateBranch(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req branchRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateBranch(&req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current int
		var code, tz string
		err := tx.QueryRow(ctx, `SELECT version,code,timezone FROM branches WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&current, &code, &tz)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("cabang")
		}
		if err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "cabang diubah pengguna lain", http.StatusConflict)
		}
		// Keep receipt numbering and business-date interpretation stable once sales exist.
		if code != req.Code || tz != req.Timezone {
			var used bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sales WHERE business_id=$1 AND branch_id=$2)`, tc.BusinessID, id).Scan(&used); err != nil {
				return err
			}
			if used {
				return errs.Conflict("kode/zona waktu cabang terkunci setelah penjualan pertama")
			}
			var open bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shifts WHERE business_id=$1 AND branch_id=$2 AND status='open')`, tc.BusinessID, id).Scan(&open); err != nil {
				return err
			}
			if open {
				return errs.Conflict("tutup shift sebelum mengubah kode/zona waktu cabang")
			}
		}
		if req.Code != code {
			return errs.Conflict("perubahan kode cabang belum didukung; buat cabang baru")
		}
		if _, err := tx.Exec(ctx, `UPDATE branches SET name=$3,timezone=$4,address=$5,receipt_header=$6,receipt_footer=$7,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Name, req.Timezone, req.Address, req.Header, req.Footer); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &id, ActorUserID: tc.UserID, Action: "branch.updated", EntityType: "branch", EntityID: id.String(), After: map[string]any{"version": v + 1, "timezone": req.Timezone}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"branch_id": id.String(), "version": v + 1})
}
func (s *Server) createRegister(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	branch, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("branch_id tidak valid"))
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" || utf8.RuneCountInString(req.Name) > 200 {
		httpx.WriteError(w, r, errs.Validation("nama terminal wajib, maksimal 200 karakter"))
		return
	}
	var reg *org.Register
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var active bool
		err := tx.QueryRow(ctx, `SELECT status='active' FROM branches WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, branch).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("cabang")
		}
		if err != nil {
			return err
		}
		if !active {
			return errs.Conflict("cabang nonaktif")
		}
		reg, err = org.NewRepo(tx).CreateRegister(ctx, tc.BusinessID, branch, strings.TrimSpace(req.Name))
		if err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, RegisterID: &reg.ID, ActorUserID: tc.UserID, Action: "register.created", EntityType: "register", EntityID: reg.ID.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"register_id": reg.ID.String(), "version": 1})
}
func (s *Server) registerStatus(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	v, e2 := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if err != nil || e2 != nil || v <= 0 {
		httpx.WriteError(w, r, errs.Validation("id/If-Match tidak valid"))
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Status != "active" && req.Status != "inactive" {
		httpx.WriteError(w, r, errs.Validation("status harus active/inactive"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		var current int
		err := tx.QueryRow(ctx, `SELECT branch_id,version FROM registers WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&branch, &current)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("terminal")
		}
		if err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "terminal diubah pengguna lain", http.StatusConflict)
		}
		if req.Status == "inactive" {
			var open bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shifts WHERE business_id=$1 AND register_id=$2 AND status='open')`, tc.BusinessID, id).Scan(&open); err != nil {
				return err
			}
			if open {
				return errs.Conflict("terminal memiliki shift terbuka; tutup shift dahulu")
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE registers SET status=$3,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Status); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, RegisterID: &id, ActorUserID: tc.UserID, Action: "register.status_changed", EntityType: "register", EntityID: id.String(), After: map[string]string{"status": req.Status}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"register_id": id.String(), "version": v + 1, "status": req.Status})
}
