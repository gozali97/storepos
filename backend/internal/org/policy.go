package org

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

type Policy struct {
	ReturnWindowDays          int   `json:"return_window_days"` // 0 = unlimited
	CashRoundingEnabled       bool  `json:"cash_rounding_enabled"`
	CashRoundingMultipleCents int64 `json:"cash_rounding_multiple_cents,string"`
	CashierDiscountBps        int64 `json:"cashier_discount_bps"`
	CashOutApprovalCents      int64 `json:"cash_out_approval_cents,string"`
	VarianceApprovalCents     int64 `json:"variance_approval_cents,string"`
}

func DefaultPolicy() Policy {
	return Policy{CashRoundingEnabled: true, CashRoundingMultipleCents: 10000, CashierDiscountBps: 1000, CashOutApprovalCents: 50000000, VarianceApprovalCents: 1000000}
}
func (p Policy) Validate() error {
	if p.ReturnWindowDays < 0 || p.ReturnWindowDays > 3650 {
		return errors.New("batas retur harus 0..3650 hari; 0 berarti tanpa batas")
	}
	if p.CashRoundingMultipleCents < 100 || p.CashRoundingMultipleCents > 100000 || p.CashRoundingMultipleCents%100 != 0 {
		return errors.New("kelipatan pembulatan harus Rp1..Rp1.000 dalam sen, tanpa pecahan rupiah")
	}
	if p.CashierDiscountBps < 0 || p.CashierDiscountBps > 10000 || p.CashOutApprovalCents < 0 || p.VarianceApprovalCents < 0 || p.CashOutApprovalCents > 100000000000 || p.VarianceApprovalCents > 100000000000 {
		return errors.New("batas diskon harus 0..10000 bps; ambang kas harus 0..Rp1 miliar")
	}
	return nil
}
func LoadPolicy(ctx context.Context, db database.Getter, biz idkit.ID) (Policy, error) {
	p := DefaultPolicy()
	var saved []byte
	if err := db.QueryRow(ctx, `SELECT settings->'operational_policy' FROM businesses WHERE id=$1`, biz).Scan(&saved); err != nil {
		return p, err
	}
	if len(saved) > 0 && string(saved) != "null" {
		// Decode onto defaults so policies saved before new fields retain previous behavior.
		if err := json.Unmarshal(saved, &p); err != nil {
			return p, err
		}
	}
	return p, p.Validate()
}
