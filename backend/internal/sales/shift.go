package sales

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/org"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

var (
	ErrApprovalRequired = errors.New("diperlukan persetujuan atasan")
	ErrReasonRequired   = errors.New("alasan wajib diisi")
	ErrInvalidAmount    = errors.New("nominal tidak valid")
	ErrNotShiftOwner    = errors.New("shift dimiliki pengguna lain")
)

// lockOpenShift mengunci shift terbuka milik terminal. FOR SHARE dipakai transaksi biasa
// (penjualan, kas); FOR UPDATE dipakai tutup shift agar menunggu semua transaksi berjalan.
func lockOpenShift(ctx context.Context, db database.Getter, biz, branch, register idkit.ID, mode string) (id, openedBy idkit.ID, opening int64, err error) {
	err = db.QueryRow(ctx,
		`SELECT id, opened_by, opening_cash_cents FROM shifts
		 WHERE business_id=$1 AND branch_id=$2 AND register_id=$3 AND status='open' `+mode,
		biz, branch, register).Scan(&id, &openedBy, &opening)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrShiftNotOpen
	} else if err != nil {
		err = fmt.Errorf("sales: baca shift: %w", err)
	}
	return
}

// CashMove mencatat kas masuk/keluar pada shift terbuka (CORE-025).
func CashMove(ctx context.Context, db database.Getter, biz, branch, register, user idkit.ID, kind string, amount int64, reason string, approvedBy *idkit.ID) error {
	if kind != "in" && kind != "out" {
		return ErrInvalidAmount
	}
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if strings.TrimSpace(reason) == "" {
		return ErrReasonRequired
	}
	policy, err := org.LoadPolicy(ctx, db, biz)
	if err != nil {
		return err
	}
	if kind == "out" && amount > policy.CashOutApprovalCents && approvedBy == nil {
		return ErrApprovalRequired
	}
	shiftID, _, _, err := lockOpenShift(ctx, db, biz, branch, register, "FOR SHARE")
	if err != nil {
		return err
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO cash_movements (id,business_id,shift_id,kind,amount_cents,reason,approved_by,created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		idkit.New(), biz, shiftID, kind, amount, strings.TrimSpace(reason), approvedBy, user); err != nil {
		return fmt.Errorf("sales: catat kas: %w", err)
	}
	return audit.Write(ctx, db, audit.Entry{BusinessID: biz, BranchID: &branch, RegisterID: &register, ActorUserID: user,
		ApprovedBy: approvedBy, Action: "cash." + kind, EntityType: "shift", EntityID: shiftID.String(),
		After: map[string]any{"amount_cents": amount}, Reason: reason})
}

type CloseResult struct {
	ShiftID                                    idkit.ID
	ExpectedCents, CountedCents, VarianceCents int64
}

// CloseShift menutup shift: kas diharapkan = kas awal + tunai neto + kas masuk − kas keluar (CORE-026).
// Refund tunai dikurangi pada shift refund; force=true untuk tutup paksa oleh Manajer/Pemilik (E7).
func CloseShift(ctx context.Context, db database.Getter, biz, branch, register, user idkit.ID, force bool, counted int64, note string, approvedBy *idkit.ID) (*CloseResult, error) {
	if counted < 0 {
		return nil, ErrInvalidAmount
	}
	shiftID, openedBy, opening, err := lockOpenShift(ctx, db, biz, branch, register, "FOR UPDATE")
	if err != nil {
		return nil, err
	}
	if openedBy != user && !force {
		return nil, ErrNotShiftOwner
	}
	var cash, in, out int64
	if err := db.QueryRow(ctx,
		`SELECT COALESCE((SELECT SUM(p.amount_cents) FROM payments p JOIN sales s ON s.business_id=p.business_id AND s.id=p.sale_id
		                  WHERE s.shift_id=$1 AND p.method='cash' AND s.status='completed' AND p.status='confirmed'),0),
		        COALESCE((SELECT SUM(amount_cents) FROM cash_movements WHERE shift_id=$1 AND kind='in'),0),
		        COALESCE((SELECT SUM(amount_cents) FROM cash_movements WHERE shift_id=$1 AND kind='out'),0)`, shiftID).
		Scan(&cash, &in, &out); err != nil {
		return nil, fmt.Errorf("sales: hitung kas: %w", err)
	}
	var refunded int64
	if err := db.QueryRow(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM refunds WHERE business_id=$1 AND shift_id=$2 AND method='cash'`, biz, shiftID).Scan(&refunded); err != nil {
		return nil, err
	}
	expected := opening + cash + in - out - refunded
	variance := counted - expected
	abs := variance
	if abs < 0 {
		abs = -abs
	}
	policy, err := org.LoadPolicy(ctx, db, biz)
	if err != nil {
		return nil, err
	}
	if abs > policy.VarianceApprovalCents {
		if strings.TrimSpace(note) == "" {
			return nil, ErrReasonRequired
		}
		if approvedBy == nil {
			return nil, ErrApprovalRequired
		}
	}
	if _, err := db.Exec(ctx,
		`UPDATE shifts SET status='closed', closed_at=now(), counted_cash_cents=$2, expected_cash_cents=$3,
		   variance_cents=$4, close_note=$5, closed_by=$6, close_approved_by=$7 WHERE id=$1`,
		shiftID, counted, expected, variance, strings.TrimSpace(note), user, approvedBy); err != nil {
		return nil, fmt.Errorf("sales: tutup shift: %w", err)
	}
	if err := audit.Write(ctx, db, audit.Entry{BusinessID: biz, BranchID: &branch, RegisterID: &register, ActorUserID: user,
		ApprovedBy: approvedBy, Action: "shift.closed", EntityType: "shift", EntityID: shiftID.String(),
		After: map[string]any{"expected_cents": expected, "counted_cents": counted, "variance_cents": variance}, Reason: note}); err != nil {
		return nil, err
	}
	return &CloseResult{ShiftID: shiftID, ExpectedCents: expected, CountedCents: counted, VarianceCents: variance}, nil
}
