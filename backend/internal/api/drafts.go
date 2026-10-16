package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
)

// Draft hanya menyimpan keranjang; PIN persetujuan dan pembayaran tidak disimpan.
type draftLine struct {
	UnitID    string      `json:"unit_id,omitempty"`
	VariantID string      `json:"variant_id"`
	QtyMilli  string      `json:"qty_milli"`
	Note      string      `json:"note"`
	Discount  discountReq `json:"discount"`
}
type draftPayload struct {
	Lines          []draftLine `json:"lines"`
	Note           string      `json:"note"`
	Discount       discountReq `json:"discount"`
	DiscountReason string      `json:"discount_reason"`
}
type draftDTO struct {
	ID         string       `json:"id"`
	BranchID   string       `json:"branch_id"`
	RegisterID string       `json:"register_id"`
	Status     string       `json:"status"`
	Version    int          `json:"version"`
	Cart       draftPayload `json:"cart"`
}

func (s *Server) draftRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/drafts", s.guard(authz.PermSaleCreate, true, s.createDraft))
	mux.Handle("GET /api/v1/drafts", s.guard(authz.PermSaleCreate, false, s.listDrafts))
	mux.Handle("PUT /api/v1/drafts/{id}", s.guard(authz.PermSaleCreate, true, s.updateDraft))
}

func validateDraft(p draftPayload) error {
	if utf8.RuneCountInString(p.Note) > 200 || utf8.RuneCountInString(p.DiscountReason) > 200 || len(p.Lines) > 200 {
		return errs.Validation("maksimal 200 baris dan 200 karakter catatan")
	}
	if _, err := p.Discount.parse(); err != nil {
		return errs.Validation(err.Error())
	}
	for _, l := range p.Lines {
		if _, err := idkit.Parse(l.VariantID); err != nil {
			return errs.Validation("variant_id tidak valid")
		}
		q, err := money.ParseMilli(l.QtyMilli)
		if err != nil || q <= 0 || utf8.RuneCountInString(l.Note) > 200 {
			return errs.Validation("kuantitas harus positif dan catatan maksimal 200 karakter")
		}
		if _, err := l.Discount.parse(); err != nil {
			return errs.Validation(err.Error())
		}
	}
	return nil
}

// Terminal dikunci lebih dahulu pada semua mutasi supaya batas hold tetap benar dalam permintaan paralel.
func lockDraftTerminal(ctx context.Context, tx database.Tx, tc tenant.Context, branch, reg idkit.ID) error {
	var active bool
	err := tx.QueryRow(ctx, `SELECT r.status='active' AND b.status='active' FROM registers r
 JOIN branches b ON b.business_id=r.business_id AND b.id=r.branch_id
 WHERE r.business_id=$1 AND r.branch_id=$2 AND r.id=$3 FOR UPDATE OF r`, tc.BusinessID, branch, reg).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.NotFound("terminal")
	}
	if err != nil {
		return err
	}
	if !active {
		return errs.Conflict("terminal atau cabang nonaktif")
	}
	if !tc.HasBranch(branch) {
		return errs.PermissionDenied("")
	}
	return nil
}

func (s *Server) createDraft(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		BranchID   string       `json:"branch_id"`
		RegisterID string       `json:"register_id"`
		Cart       draftPayload `json:"cart"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, e1 := idkit.Parse(req.BranchID)
	reg, e2 := idkit.Parse(req.RegisterID)
	if e1 != nil || e2 != nil {
		httpx.WriteError(w, r, errs.Validation("branch_id/register_id tidak valid"))
		return
	}
	if err := validateDraft(req.Cart); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	id := idkit.New()
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if err := lockDraftTerminal(ctx, tx, tc, branch, reg); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sale_drafts WHERE business_id=$1 AND register_id=$2 AND status='active')`, tc.BusinessID, reg).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return errs.Conflict("terminal sudah memiliki draft aktif; tahan draft tersebut terlebih dahulu")
		}
		if err := validateDraftVariants(ctx, tx, tc.BusinessID, req.Cart); err != nil {
			return err
		}
		payload, err := json.Marshal(req.Cart)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO sale_drafts(id,business_id,branch_id,register_id,payload,created_by) VALUES($1,$2,$3,$4,$5,$6)`, id, tc.BusinessID, branch, reg, payload, tc.UserID)
		return err
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, draftDTO{ID: id.String(), BranchID: branch.String(), RegisterID: reg.String(), Status: "active", Version: 1, Cart: req.Cart})
}

func (s *Server) listDrafts(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	reg, err := idkit.Parse(r.URL.Query().Get("register_id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("register_id wajib diisi"))
		return
	}
	var branch idkit.ID
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT branch_id FROM registers WHERE business_id=$1 AND id=$2`, tc.BusinessID, reg).Scan(&branch)
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
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,status,version,payload FROM sale_drafts WHERE business_id=$1 AND register_id=$2 AND status IN ('active','held') ORDER BY created_at`, tc.BusinessID, reg)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	out := []draftDTO{}
	for rows.Next() {
		d := draftDTO{BranchID: branch.String(), RegisterID: reg.String()}
		var raw []byte
		if err := rows.Scan(&d.ID, &d.Status, &d.Version, &raw); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if err := json.Unmarshal(raw, &d.Cart); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"drafts": out})
}

func (s *Server) updateDraft(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, e1 := idkit.Parse(r.PathValue("id"))
	version, e2 := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if e1 != nil || e2 != nil || version <= 0 {
		httpx.WriteError(w, r, errs.Validation("id dan If-Match versi positif wajib diisi"))
		return
	}
	var req struct {
		Status string       `json:"status"`
		Cart   draftPayload `json:"cart"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Status != "active" && req.Status != "held" && req.Status != "cancelled" {
		httpx.WriteError(w, r, errs.Validation("status harus active/held/cancelled"))
		return
	}
	if err := validateDraft(req.Cart); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var d draftDTO
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch, reg idkit.ID
		err := tx.QueryRow(ctx, `SELECT branch_id,register_id FROM sale_drafts WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&branch, &reg)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("draft")
		}
		if err != nil {
			return err
		}
		if err := lockDraftTerminal(ctx, tx, tc, branch, reg); err != nil {
			return err
		}
		var status string
		var current int
		if err := tx.QueryRow(ctx, `SELECT status,version FROM sale_drafts WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&status, &current); err != nil {
			return err
		}
		if current != version {
			return errs.New(errs.CodeOptimisticLockFailure, "draft diubah pengguna lain", http.StatusConflict)
		}
		if status != "active" && status != "held" {
			return errs.Conflict("draft sudah selesai atau dibatalkan")
		}
		if req.Status == "held" && status != "held" {
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM sale_drafts WHERE business_id=$1 AND register_id=$2 AND status='held'`, tc.BusinessID, reg).Scan(&n); err != nil {
				return err
			}
			if n >= 10 {
				return errs.Conflict("maksimal 10 draft tertahan per terminal")
			}
		}
		if req.Status == "active" && status != "active" {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sale_drafts WHERE business_id=$1 AND register_id=$2 AND status='active')`, tc.BusinessID, reg).Scan(&exists); err != nil {
				return err
			}
			if exists {
				return errs.Conflict("tahan draft aktif sebelum melanjutkan draft ini")
			}
		}
		if req.Status != "cancelled" {
			if err := validateDraftVariants(ctx, tx, tc.BusinessID, req.Cart); err != nil {
				return err
			}
		}
		payload, err := json.Marshal(req.Cart)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE sale_drafts SET status=$3,payload=$4,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Status, payload); err != nil {
			return err
		}
		d = draftDTO{ID: id.String(), BranchID: branch.String(), RegisterID: reg.String(), Status: req.Status, Version: current + 1, Cart: req.Cart}
		return nil
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}
