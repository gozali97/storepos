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

func (s *Server) onboardingRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/onboarding", s.guard(authz.PermBusinessManage, false, s.getOnboarding))
	mux.Handle("POST /api/v1/onboarding/events", s.guard(authz.PermBusinessManage, true, s.recordOnboardingEvent))
	mux.Handle("PUT /api/v1/onboarding/dismiss", s.guard(authz.PermBusinessManage, true, s.dismissOnboarding))
	mux.Handle("POST /api/v1/onboarding/demo", s.guard(authz.PermProductManage, true, s.createDemo))
	mux.Handle("DELETE /api/v1/onboarding/demo", s.guard(authz.PermProductManage, true, s.removeDemo))
}

type demoItem struct {
	name, sku, unit string
	price           int64 // sen
}

// Contoh data per jenis usaha. Apotek memakai barang non-obat agar tidak ada kesan klasifikasi/izin edar,
// dan semua tanpa pelacakan stok sehingga langsung bisa dipakai untuk transaksi uji.
var demoCatalog = map[string][]demoItem{
	"retail": {
		{"[Contoh] Air Mineral 600 ml", "DEMO-AIR", "botol", 400000},
		{"[Contoh] Kopi Susu Botol", "DEMO-KOPI", "botol", 1200000},
		{"[Contoh] Beras Premium 5 kg", "DEMO-BERAS", "karung", 7800000},
	},
	"restaurant": {
		{"[Contoh] Nasi Goreng Spesial", "DEMO-NASGOR", "porsi", 2800000},
		{"[Contoh] Es Teh Manis", "DEMO-ESTEH", "gelas", 800000},
		{"[Contoh] Ayam Bakar", "DEMO-AYAM", "porsi", 3200000},
	},
	"pharmacy": {
		{"[Contoh] Masker Medis", "DEMO-MASKER", "pcs", 250000},
		{"[Contoh] Termometer Digital", "DEMO-TERMO", "pcs", 4500000},
		{"[Contoh] Plester Luka", "DEMO-PLESTER", "box", 1500000},
	},
}

// createDemo membuat atau memulihkan produk contoh. Idempoten: pemanggilan ulang tidak menggandakan data.
func (s *Server) createDemo(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var created, restored int
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		created, restored = 0, 0
		var typ string
		if err := tx.QueryRow(ctx, `SELECT business_type FROM businesses WHERE id=$1 FOR UPDATE`, tc.BusinessID).Scan(&typ); err != nil {
			return err
		}
		for _, it := range demoCatalog[typ] {
			var pid idkit.ID
			var demo bool
			err := tx.QueryRow(ctx, `SELECT p.id,p.is_demo FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id WHERE v.business_id=$1 AND lower(v.sku)=lower($2)`, tc.BusinessID, it.sku).Scan(&pid, &demo)
			switch {
			case err == nil && !demo:
				return errs.Conflict("SKU " + it.sku + " sudah dipakai produk Anda; ubah SKU tersebut dulu")
			case err == nil:
				tag, e := tx.Exec(ctx, `UPDATE products SET status='active',version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2 AND status='archived'`, tc.BusinessID, pid)
				if e != nil {
					return e
				}
				if tag.RowsAffected() == 1 {
					restored++
					if _, e := tx.Exec(ctx, `UPDATE variants SET status='active',updated_at=now() WHERE business_id=$1 AND product_id=$2`, tc.BusinessID, pid); e != nil {
						return e
					}
				}
			case errors.Is(err, pgx.ErrNoRows):
				p := &catalog.Product{Name: it.name, TrackStock: false}
				if e := catalog.NewRepo(tx).CreateProduct(ctx, tc.BusinessID, p, []catalog.NewVariant{{SKU: it.sku, BaseUnit: it.unit, PriceCents: it.price}}); e != nil {
					return e
				}
				if _, e := tx.Exec(ctx, `UPDATE products SET is_demo=true WHERE business_id=$1 AND id=$2`, tc.BusinessID, p.ID); e != nil {
					return e
				}
				created++
			default:
				return err
			}
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "onboarding.demo_created", EntityType: "business", EntityID: tc.BusinessID.String(), After: map[string]int{"created": created, "restored": restored}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]int{"created": created, "restored": restored})
}

// removeDemo mengarsipkan semua produk contoh. Tidak ada hapus fisik: transaksi uji yang memakainya tetap utuh.
func (s *Server) removeDemo(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var archived int64
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var inUse bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM restaurant_order_lines l
 JOIN restaurant_orders o ON o.business_id=l.business_id AND o.id=l.order_id
 JOIN variants v ON v.business_id=l.business_id AND v.id=l.variant_id
 JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id
 WHERE l.business_id=$1 AND p.is_demo AND o.status='open' AND l.kitchen_status<>'CANCELLED')`, tc.BusinessID).Scan(&inUse); err != nil {
			return err
		}
		if inUse {
			return errs.Conflict("menu contoh masih ada di pesanan terbuka; selesaikan atau batalkan pesanan itu dulu")
		}
		if _, err := tx.Exec(ctx, `UPDATE variants v SET status='archived',updated_at=now() FROM products p WHERE p.business_id=v.business_id AND p.id=v.product_id AND p.business_id=$1 AND p.is_demo AND v.status<>'archived'`, tc.BusinessID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE products SET status='archived',version=version+1,updated_at=now() WHERE business_id=$1 AND is_demo AND status<>'archived'`, tc.BusinessID)
		if err != nil {
			return err
		}
		archived = tag.RowsAffected()
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "onboarding.demo_removed", EntityType: "business", EntityID: tc.BusinessID.String(), After: map[string]int64{"archived": archived}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int64{"archived": archived})
}

type checklistItem struct {
	Key  string `json:"key"`
	Done bool   `json:"done"`
}

// getOnboarding menurunkan progres dari data nyata, bukan flag manual, sehingga tidak bisa tertinggal dari kenyataan.
func (s *Server) getOnboarding(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var created time.Time
	var firstSale *time.Time
	var dismissed, branch, product, demo, team, shift bool
	err := s.pool.Underlying().QueryRow(r.Context(), `SELECT b.created_at,b.first_sale_completed_at,COALESCE((b.settings->>'onboarding_dismissed')::boolean,false),
 EXISTS(SELECT 1 FROM branches WHERE business_id=b.id AND status='active' AND COALESCE(address,'')<>''),
 EXISTS(SELECT 1 FROM products WHERE business_id=b.id AND NOT is_demo AND status='active'),
 EXISTS(SELECT 1 FROM products WHERE business_id=b.id AND is_demo AND status='active'),
 EXISTS(SELECT 1 FROM memberships WHERE business_id=b.id AND role<>'owner') OR EXISTS(SELECT 1 FROM invitations WHERE business_id=b.id),
 EXISTS(SELECT 1 FROM shifts WHERE business_id=b.id)
 FROM businesses b WHERE b.id=$1`, tc.BusinessID).Scan(&created, &firstSale, &dismissed, &branch, &product, &demo, &team, &shift)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := []checklistItem{
		{"branch_address", branch}, {"first_product", product}, {"invite_team", team},
		{"open_shift", shift}, {"first_sale", firstSale != nil},
	}
	done := 0
	for _, it := range items {
		if it.Done {
			done++
		}
	}
	// M2 (PRD §3): menit dari bisnis dibuat sampai penjualan pertama; nil sebelum ada penjualan.
	var m2 *int64
	if firstSale != nil {
		v := int64(firstSale.Sub(created).Minutes())
		if v < 0 {
			v = 0
		}
		m2 = &v
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "done": done, "total": len(items), "dismissed": dismissed, "demo_present": demo, "m2_minutes": m2})
}

func (s *Server) dismissOnboarding(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		Dismissed *bool `json:"dismissed"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Dismissed == nil {
		httpx.WriteError(w, r, errs.Validation("dismissed wajib boolean"))
		return
	}
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE businesses SET settings=jsonb_set(settings,'{onboarding_dismissed}',to_jsonb($2::boolean),true) WHERE id=$1`, tc.BusinessID, *req.Dismissed); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "onboarding.dismissed_changed", EntityType: "business", EntityID: tc.BusinessID.String(), After: map[string]bool{"dismissed": *req.Dismissed}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"dismissed": *req.Dismissed})
}

// recordOnboardingEvent hanya menerima milestone terbatas, tanpa data formulir/PII.
// Satu event per langkah/status/bisnis; lock tenant mencegah duplikasi retry paralel.
func (s *Server) recordOnboardingEvent(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		Step   string `json:"step"`
		Status string `json:"status"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	switch req.Step {
	case "business", "type", "branch", "settings", "product":
	default:
		httpx.WriteError(w, r, errs.Validation("langkah onboarding tidak valid"))
		return
	}
	if req.Status != "completed" && req.Status != "skipped" {
		httpx.WriteError(w, r, errs.Validation("status onboarding tidak valid"))
		return
	}
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT id FROM businesses WHERE id=$1 FOR UPDATE`, tc.BusinessID); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM audit_logs WHERE business_id=$1 AND action='onboarding.step' AND entity_type='onboarding' AND entity_id=$2 AND after_jsonb->>'status'=$3)`, tc.BusinessID, req.Step, req.Status).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "onboarding.step", EntityType: "onboarding", EntityID: req.Step, After: map[string]string{"status": req.Status}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
