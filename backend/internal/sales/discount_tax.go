package sales

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/pricing"
)

// validDiscount: nominal > 0; persen 1..10000 bps (CORE-015).
func validDiscount(d pricing.Discount) bool {
	switch d.Kind {
	case pricing.DiscountNone:
		return d.Value == 0
	case pricing.DiscountAmount:
		return d.Value > 0
	case pricing.DiscountPercent:
		return d.Value > 0 && d.Value <= 10000
	}
	return false
}

// checkDiscounts menolak diskon nominal yang melebihi nilainya (mesin harga hanya memotongnya) dan
// menegakkan batas diskon per peran: di atas batas wajib ada persetujuan atasan beserta alasan.
func checkDiscounts(in Input, t pricing.Totals) error {
	for i, l := range in.Lines {
		if l.Discount.Kind == pricing.DiscountAmount && l.Discount.Value > int64(t.Lines[i].GrossCents) {
			return ErrDiscountExceeds
		}
	}
	if in.TxDiscount.Kind == pricing.DiscountAmount {
		var base int64
		for _, l := range t.Lines {
			base += int64(l.NetBeforeTxCents)
		}
		if in.TxDiscount.Value > base {
			return ErrDiscountExceeds
		}
	}
	total := int64(t.TotalLineDiscountCents + t.TransactionDiscountCents)
	if total == 0 || t.SubtotalGrossCents <= 0 {
		return nil
	}
	// Bandingkan rasio tepat, tanpa overflow atau pemotongan bps yang dapat meloloskan diskon di atas batas.
	actual := new(big.Int).Mul(big.NewInt(total), big.NewInt(10000))
	limit := new(big.Int).Mul(big.NewInt(int64(t.SubtotalGrossCents)), big.NewInt(in.DiscountLimitBps))
	over := !in.UnlimitedDiscount && actual.Cmp(limit) > 0
	if over && in.DiscountApprovedBy == nil {
		return ErrApprovalRequired
	}
	if in.DiscountApprovedBy != nil && strings.TrimSpace(in.DiscountReason) == "" {
		return ErrReasonRequired
	}
	return nil
}

// loadTax membaca aturan pajak aktif bisnis; nil = tidak ada pajak.
// shortcut: satu aturan untuk semua produk; cakupan per kategori/produk menyusul (CORE-016).
func loadTax(ctx context.Context, db database.Getter, biz idkit.ID) (*pricing.TaxRule, error) {
	var name, mode string
	var bps int64
	err := db.QueryRow(ctx, `SELECT name, rate_bps, mode FROM tax_rules WHERE business_id=$1 AND active`, biz).Scan(&name, &bps, &mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sales: baca aturan pajak: %w", err)
	}
	r := &pricing.TaxRule{Name: name, RateBps: bps, Mode: pricing.TaxExclusive}
	if mode == "inclusive" {
		r.Mode = pricing.TaxInclusive
	}
	return r, nil
}

var ErrInvalidTax = errors.New("aturan pajak tidak valid")

// SetTaxRule mengganti aturan pajak aktif. Hanya berlaku untuk transaksi baru (snapshot).
// Sistem tidak menentukan tarif; Pemilik yang mengisi (CORE-016 [VAL]).
func SetTaxRule(ctx context.Context, db database.Getter, biz, user idkit.ID, name string, rateBps int, mode string) error {
	if strings.TrimSpace(name) == "" || rateBps < 0 || rateBps > 10000 || (mode != "exclusive" && mode != "inclusive") {
		return ErrInvalidTax
	}
	if _, err := db.Exec(ctx, `UPDATE tax_rules SET active=false WHERE business_id=$1 AND active`, biz); err != nil {
		return fmt.Errorf("sales: nonaktifkan pajak lama: %w", err)
	}
	id := idkit.New()
	if _, err := db.Exec(ctx, `INSERT INTO tax_rules (id,business_id,name,rate_bps,mode) VALUES ($1,$2,$3,$4,$5)`,
		id, biz, strings.TrimSpace(name), rateBps, mode); err != nil {
		return fmt.Errorf("sales: simpan pajak: %w", err)
	}
	return audit.Write(ctx, db, audit.Entry{BusinessID: biz, ActorUserID: user, Action: "tax_rule.changed",
		EntityType: "tax_rule", EntityID: id.String(),
		After: map[string]any{"name": name, "rate_bps": rateBps, "mode": mode}})
}
