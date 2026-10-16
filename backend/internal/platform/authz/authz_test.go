package authz

import (
	"errors"
	"testing"

	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func TestMatrixSummary(t *testing.T) {
	// Owner mendapat izin membuat penjualan.
	if Check(tenant.RoleOwner, PermSaleCreate) != Allowed {
		t.Error("owner sale.create harus Allowed")
	}
	// Cashier perlu O untuk void (§7.2).
	if g := Check(tenant.RoleCashier, PermSaleVoid); g != AllowedWithO {
		t.Errorf("cashier void = %v, want AllowedWithO", g)
	}
	// Warehouse tidak boleh membuat penjualan.
	if Check(tenant.RoleWarehouse, PermSaleCreate) != Denied {
		t.Error("warehouse sale.create harus Denied")
	}
	// Kitchen hanya update dapur.
	if Check(tenant.RoleKitchen, PermSaleCreate) != Denied {
		t.Error("kitchen sale.create harus Denied")
	}
	if Check(tenant.RoleKitchen, PermKitchenUpdate) != Allowed {
		t.Error("kitchen update harus Allowed")
	}
	// Platform admin: default tanpa akses operasi tenant.
	if Check(tenant.RolePlatformAdmin, PermSaleCreate) != Denied {
		t.Error("platform admin sale.create harus Denied")
	}
	// Pemilik laporan laba (default); Manajer belum berhak (TBK).
	if Check(tenant.RoleOwner, PermReportProfit) != Allowed {
		t.Error("owner laporan laba Allowed")
	}
	if Check(tenant.RoleManager, PermReportProfit) != Denied {
		t.Error("manager laporan laba default Denied")
	}
}

func TestRequireBranchAssignment(t *testing.T) {
	b1 := idkit.NewV7()
	b2 := idkit.NewV7()
	tc := tenant.Context{
		BusinessID: idkit.NewV7(),
		Role:       tenant.RoleCashier,
		BranchIDs:  []idkit.ID{b1},
	}
	if _, err := Require(tc, PermSaleCreate, b1); err != nil {
		t.Fatalf("cabang ditugaskan harus diizinkan: %v", err)
	}
	if _, err := Require(tc, PermSaleCreate, b2); err == nil {
		t.Fatal("cabang di luar penugasan harus ditolak")
	}
	var de *DeniedError
	_, err := Require(tc, PermSaleCreate, b2)
	if !errors.As(err, &de) {
		t.Fatalf("tipe error = %T", err)
	}
}

func TestRequireOwnerIgnoresBranch(t *testing.T) {
	tc := tenant.Context{
		BusinessID: idkit.NewV7(),
		Role:       tenant.RoleOwner,
	}
	random := idkit.NewV7()
	if _, err := Require(tc, PermSaleCreate, random); err != nil {
		t.Fatalf("owner harus bisa ke cabang mana pun: %v", err)
	}
}

// Uji matriks: setiap kombinasi role × permission dicek konsistensinya.
// Kombinasi "Allowed" tidak boleh ditandai sekaligus "Denied" dsb — dicegah tipe.
// Uji ini mendeteksi kelupaan registrasi: minimal Owner punya izin inti.
func TestOwnerCoreCoverage(t *testing.T) {
	coreForOwner := []Permission{
		PermBusinessManage, PermBranchManage, PermUserManage,
		PermProductManage, PermSaleCreate, PermShiftManageOwn,
		PermGoodsReceipt, PermStockAdjust, PermReportSales, PermReportProfit, PermAuditView,
	}
	for _, p := range coreForOwner {
		if Check(tenant.RoleOwner, p) != Allowed {
			t.Errorf("owner %s harus Allowed", p)
		}
	}
}
