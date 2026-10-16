// Package authz adalah sumber kebenaran untuk daftar izin dan matriks peran×izin
// sesuai PRD §7.2. Suite uji otomatis dibangkitkan dari paket ini (CORE-005).
package authz

import (
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

// Permission mengidentifikasi satu tindakan yang dapat diizinkan atau ditolak.
type Permission string

// Daftar izin inti. Nama permission dirancang mesin-dapat-diuraikan: <domain>.<aksi>.
const (
	// Pengaturan & akses
	PermBusinessManage    Permission = "business.manage"
	PermBranchManage      Permission = "branch.manage"
	PermBranchView        Permission = "branch.view"
	PermUserManage        Permission = "user.manage"
	PermUserManageLimited Permission = "user.manage.limited" // Manajer: tidak bisa Pemilik/Manajer

	// Produk
	PermProductManage   Permission = "product.manage"
	PermProductView     Permission = "product.view"
	PermProductViewCost Permission = "product.view_cost"

	// Transaksi
	PermSaleCreate          Permission = "sale.create"
	PermDiscountWithinLimit Permission = "sale.discount.within_limit"
	PermDiscountOverride    Permission = "sale.discount.override" // melebihi batas, perlu O
	PermSaleVoid            Permission = "sale.void"
	PermSaleReturn          Permission = "sale.return"

	// Shift & kas
	PermShiftManageOwn Permission = "shift.manage_own"
	PermShiftViewAll   Permission = "shift.view_all"
	PermCashInOut      Permission = "shift.cash_in_out"

	// Stok
	PermGoodsReceipt    Permission = "stock.receive"
	PermStockAdjust     Permission = "stock.adjust"
	PermStockCountInput Permission = "stock.count_input"
	PermStockCountPost  Permission = "stock.count_post"
	PermSupplierManage  Permission = "supplier.manage"

	// Keuangan
	PermExpenseManage Permission = "expense.manage"

	// Laporan
	PermReportSales  Permission = "report.sales"
	PermReportProfit Permission = "report.profit"

	// Restoran
	PermKitchenUpdate       Permission = "kitchen.update_status"
	PermTableManage         Permission = "table.manage"
	PermCancelItemAfterSend Permission = "sale.cancel_item_after_send"

	// Apotek
	PermBatchManage        Permission = "pharmacy.batch_manage"
	PermPrescriptionVerify Permission = "pharmacy.prescription_verify"
	PermQuarantineRelease  Permission = "pharmacy.quarantine_release"

	// Audit & platform
	PermAuditView       Permission = "audit.view"
	PermPlatformSupport Permission = "platform.support_access"
)

// Grant adalah hasil otorisasi untuk satu permission.
type Grant int

const (
	Denied       Grant = iota // ✗
	Allowed                   // ✓
	AllowedWithO              // O: perlu otorisasi atasan (PIN) sebelum eksekusi
	ReadOnly                  // L: hanya lihat
)

// matrix memetakan (Role, Permission) ke Grant.
// Nilai default (tanpa entri) = Denied.
var matrix = map[tenant.Role]map[Permission]Grant{}

func set(role tenant.Role, perm Permission, g Grant) {
	if matrix[role] == nil {
		matrix[role] = map[Permission]Grant{}
	}
	matrix[role][perm] = g
}

func init() {
	// Owner: ✓ untuk semua kecuali tindakan farmasi yang perlu validator (PRD §7.2 catatan).
	all := []Permission{
		PermBusinessManage, PermBranchManage, PermBranchView, PermUserManage,
		PermProductManage, PermProductView, PermProductViewCost,
		PermSaleCreate, PermDiscountWithinLimit, PermDiscountOverride, PermSaleVoid, PermSaleReturn,
		PermShiftManageOwn, PermShiftViewAll, PermCashInOut,
		PermGoodsReceipt, PermStockAdjust, PermStockCountInput, PermStockCountPost, PermSupplierManage,
		PermExpenseManage, PermReportSales, PermReportProfit,
		PermKitchenUpdate, PermTableManage, PermCancelItemAfterSend,
		PermBatchManage, PermQuarantineRelease,
		PermAuditView,
	}
	for _, p := range all {
		set(tenant.RoleOwner, p, Allowed)
	}
	// Verifikasi resep: default Denied untuk Owner (perlu validator); diizinkan Apoteker.
	// Lihat PRD §7.2 catatan.

	// Manager: ✓ untuk operasional, L untuk pengaturan bisnis/cabang, tanpa laporan laba.
	set(tenant.RoleManager, PermBranchView, ReadOnly)
	set(tenant.RoleManager, PermUserManageLimited, Allowed)
	set(tenant.RoleManager, PermProductManage, Allowed)
	set(tenant.RoleManager, PermProductView, Allowed)
	set(tenant.RoleManager, PermProductViewCost, Allowed)
	set(tenant.RoleManager, PermSaleCreate, Allowed)
	set(tenant.RoleManager, PermDiscountWithinLimit, Allowed)
	set(tenant.RoleManager, PermDiscountOverride, Allowed)
	set(tenant.RoleManager, PermSaleVoid, Allowed)
	set(tenant.RoleManager, PermSaleReturn, Allowed)
	set(tenant.RoleManager, PermShiftManageOwn, Allowed)
	set(tenant.RoleManager, PermShiftViewAll, Allowed)
	set(tenant.RoleManager, PermCashInOut, Allowed)
	set(tenant.RoleManager, PermGoodsReceipt, Allowed)
	set(tenant.RoleManager, PermStockAdjust, Allowed)
	set(tenant.RoleManager, PermStockCountInput, Allowed)
	set(tenant.RoleManager, PermStockCountPost, Allowed)
	set(tenant.RoleManager, PermSupplierManage, Allowed)
	set(tenant.RoleManager, PermExpenseManage, Allowed)
	set(tenant.RoleManager, PermReportSales, Allowed)
	// PermReportProfit untuk Manager = TBK (PRD §7.2). Default Denied.
	set(tenant.RoleManager, PermKitchenUpdate, Allowed)
	set(tenant.RoleManager, PermTableManage, Allowed)
	set(tenant.RoleManager, PermCancelItemAfterSend, Allowed)
	set(tenant.RoleManager, PermAuditView, ReadOnly)

	// Cashier
	set(tenant.RoleCashier, PermProductView, ReadOnly)
	set(tenant.RoleCashier, PermSaleCreate, Allowed)
	set(tenant.RoleCashier, PermDiscountWithinLimit, Allowed)
	set(tenant.RoleCashier, PermDiscountOverride, AllowedWithO)
	set(tenant.RoleCashier, PermSaleVoid, AllowedWithO)
	set(tenant.RoleCashier, PermSaleReturn, AllowedWithO)
	set(tenant.RoleCashier, PermShiftManageOwn, Allowed)
	set(tenant.RoleCashier, PermCashInOut, Allowed)
	set(tenant.RoleCashier, PermExpenseManage, AllowedWithO) // kas keluar
	set(tenant.RoleCashier, PermTableManage, ReadOnly)       // L + pakai

	// Warehouse
	set(tenant.RoleWarehouse, PermProductView, ReadOnly) // tanpa harga beli
	set(tenant.RoleWarehouse, PermProductViewCost, Allowed)
	set(tenant.RoleWarehouse, PermGoodsReceipt, Allowed)
	set(tenant.RoleWarehouse, PermStockAdjust, AllowedWithO)
	set(tenant.RoleWarehouse, PermStockCountInput, Allowed)
	set(tenant.RoleWarehouse, PermSupplierManage, Allowed)
	set(tenant.RoleWarehouse, PermBatchManage, Allowed)

	// Kitchen
	set(tenant.RoleKitchen, PermKitchenUpdate, Allowed)
	set(tenant.RoleKitchen, PermProductView, ReadOnly) // menu

	// Pharmacist: izin Cashier + farmasi
	set(tenant.RolePharmacist, PermProductManage, Allowed) // data obat
	set(tenant.RolePharmacist, PermProductView, Allowed)
	set(tenant.RolePharmacist, PermProductViewCost, Allowed)
	set(tenant.RolePharmacist, PermSaleCreate, Allowed)
	set(tenant.RolePharmacist, PermDiscountWithinLimit, Allowed)
	set(tenant.RolePharmacist, PermDiscountOverride, AllowedWithO)
	set(tenant.RolePharmacist, PermSaleVoid, AllowedWithO)
	set(tenant.RolePharmacist, PermSaleReturn, AllowedWithO)
	set(tenant.RolePharmacist, PermShiftManageOwn, Allowed)
	set(tenant.RolePharmacist, PermCashInOut, Allowed)
	set(tenant.RolePharmacist, PermGoodsReceipt, Allowed)
	set(tenant.RolePharmacist, PermStockAdjust, AllowedWithO)
	set(tenant.RolePharmacist, PermStockCountInput, Allowed)
	set(tenant.RolePharmacist, PermSupplierManage, Allowed)
	set(tenant.RolePharmacist, PermBatchManage, Allowed)
	set(tenant.RolePharmacist, PermPrescriptionVerify, Allowed)
	set(tenant.RolePharmacist, PermQuarantineRelease, Allowed)
	set(tenant.RolePharmacist, PermReportSales, ReadOnly)

	// Platform admin: default tanpa akses tenant; akses via grant terpisah (CORE-045).
	set(tenant.RolePlatformAdmin, PermPlatformSupport, Allowed)
}

// Check mengembalikan Grant untuk kombinasi (role, perm).
func Check(role tenant.Role, perm Permission) Grant {
	if m, ok := matrix[role]; ok {
		if g, ok := m[perm]; ok {
			return g
		}
	}
	return Denied
}

// Require memeriksa izin untuk tc.Role + optional branch assignment.
// Mengembalikan nil bila diizinkan (termasuk AllowedWithO - caller harus memverifikasi PIN).
// branchID boleh nol untuk tindakan yang tidak terkait cabang.
func Require(tc tenant.Context, perm Permission, branchID idkit.ID) (Grant, error) {
	g := Check(tc.Role, perm)
	if g == Denied {
		return Denied, errDenied(perm)
	}
	if g == ReadOnly {
		return g, errDenied(perm)
	}
	if !branchID.IsZero() && !tc.HasBranch(branchID) {
		return Denied, errDenied(perm)
	}
	return g, nil
}

// RequireRead sama dengan Require tapi menerima ReadOnly sebagai izin.
func RequireRead(tc tenant.Context, perm Permission, branchID idkit.ID) error {
	g := Check(tc.Role, perm)
	if g == Denied {
		return errDenied(perm)
	}
	if !branchID.IsZero() && !tc.HasBranch(branchID) {
		return errDenied(perm)
	}
	return nil
}

type DeniedError struct{ Perm Permission }

func (e *DeniedError) Error() string { return "izin ditolak: " + string(e.Perm) }

func errDenied(p Permission) error { return &DeniedError{Perm: p} }

// AllRoles mengembalikan daftar lengkap peran untuk uji matriks.
func AllRoles() []tenant.Role {
	return []tenant.Role{
		tenant.RoleOwner, tenant.RoleManager, tenant.RoleCashier,
		tenant.RoleWarehouse, tenant.RoleKitchen, tenant.RolePharmacist,
		tenant.RolePlatformAdmin,
	}
}

// AllPermissions mengembalikan daftar seluruh permission.
func AllPermissions() []Permission {
	return []Permission{
		PermBusinessManage, PermBranchManage, PermBranchView,
		PermUserManage, PermUserManageLimited,
		PermProductManage, PermProductView, PermProductViewCost,
		PermSaleCreate, PermDiscountWithinLimit, PermDiscountOverride, PermSaleVoid, PermSaleReturn,
		PermShiftManageOwn, PermShiftViewAll, PermCashInOut,
		PermGoodsReceipt, PermStockAdjust, PermStockCountInput, PermStockCountPost, PermSupplierManage,
		PermExpenseManage, PermReportSales, PermReportProfit,
		PermKitchenUpdate, PermTableManage, PermCancelItemAfterSend,
		PermBatchManage, PermPrescriptionVerify, PermQuarantineRelease,
		PermAuditView, PermPlatformSupport,
	}
}
