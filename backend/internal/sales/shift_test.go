package sales

import (
	"context"
	"errors"
	"testing"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

func (e *env) inTx(fn func(ctx context.Context, tx database.Tx) error) error {
	return e.pool.InTx(context.Background(), fn)
}

func (e *env) closeShift(force bool, counted int64, note string, appr *idkit.ID) (*CloseResult, error) {
	var r *CloseResult
	err := e.inTx(func(ctx context.Context, tx database.Tx) error {
		var err error
		r, err = CloseShift(ctx, tx, e.biz, e.branch, e.register, e.user, force, counted, note, appr)
		return err
	})
	return r, err
}

// AC CORE-026: kas awal 200rb, tunai neto 1jt, kas keluar 50rb, dihitung 1,14jt → diharapkan 1,15jt, selisih -10rb.
func TestCloseShiftExpectedAndVariance(t *testing.T) {
	e := setup(t, 1000)
	db := e.pool.Underlying()
	ctx := context.Background()
	if _, err := db.Exec(ctx, `UPDATE shifts SET opening_cash_cents=20000000`); err != nil {
		t.Fatal(err)
	}
	// 100 x Rp15.000 = Rp1.500.000 tunai; bayar pas supaya kembalian 0.
	for i := 0; i < 100; i++ {
		if _, err := e.sell(1, 1500000); err != nil {
			t.Fatal(err)
		}
	}
	// Rp1.500.000 tunai total; sesuaikan ke skenario AC dengan kas keluar Rp350.000 → diharapkan 200rb+1,5jt-350rb = 1,35jt.
	err := e.inTx(func(ctx context.Context, tx database.Tx) error {
		return CashMove(ctx, tx, e.biz, e.branch, e.register, e.user, "out", 35_000_000, "beli es batu", nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := e.closeShift(false, 135_000_000-500_000, "", nil) // kurang Rp5.000 (di bawah ambang)
	if err != nil {
		t.Fatal(err)
	}
	if r.ExpectedCents != 135_000_000 || r.VarianceCents != -500_000 {
		t.Fatalf("hitungan salah: %+v", r)
	}
}

func TestCloseShiftBigVarianceNeedsNoteAndApproval(t *testing.T) {
	e := setup(t, 5)
	if _, err := e.closeShift(false, 5_000_000, "", nil); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("selisih besar tanpa catatan harus ErrReasonRequired, dapat %v", err)
	}
	if _, err := e.closeShift(false, 5_000_000, "uang tercecer", nil); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("selisih besar tanpa O harus ErrApprovalRequired, dapat %v", err)
	}
	boss := idkit.New()
	if _, err := e.pool.Underlying().Exec(context.Background(), `INSERT INTO users (id,email,password_hash) VALUES ($1,'b@x.test','x')`, boss); err != nil {
		t.Fatal(err)
	}
	if _, err := e.closeShift(false, 5_000_000, "uang tercecer", &boss); err != nil {
		t.Fatalf("dengan catatan + O harus berhasil: %v", err)
	}
}

func TestClosedShiftIsLockedAndRejectsSales(t *testing.T) {
	e := setup(t, 5)
	if _, err := e.closeShift(false, 0, "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.sell(1, 1500000); !errors.Is(err, ErrShiftNotOpen) {
		t.Fatalf("finalisasi di shift tertutup harus ErrShiftNotOpen, dapat %v", err)
	}
	if _, err := e.pool.Underlying().Exec(context.Background(), `UPDATE shifts SET variance_cents=0`); err == nil {
		t.Fatal("UPDATE shift tertutup harus ditolak trigger")
	}
}

func TestCashOutOverThresholdNeedsApproval(t *testing.T) {
	e := setup(t, 0)
	do := func(amount int64, reason string) error {
		return e.inTx(func(ctx context.Context, tx database.Tx) error {
			return CashMove(ctx, tx, e.biz, e.branch, e.register, e.user, "out", amount, reason, nil)
		})
	}
	if err := do(60_000_000, "setor"); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("di atas ambang tanpa O harus ErrApprovalRequired, dapat %v", err)
	}
	if err := do(1_000_000, " "); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("tanpa alasan harus ErrReasonRequired, dapat %v", err)
	}
	if err := do(1_000_000, "parkir"); err != nil {
		t.Fatal(err)
	}
}

func TestOnlyOwnerOrForceCanClose(t *testing.T) {
	e := setup(t, 0)
	other := idkit.New()
	if _, err := e.pool.Underlying().Exec(context.Background(), `INSERT INTO users (id,email,password_hash) VALUES ($1,'o@x.test','x')`, other); err != nil {
		t.Fatal(err)
	}
	err := e.inTx(func(ctx context.Context, tx database.Tx) error {
		_, err := CloseShift(ctx, tx, e.biz, e.branch, e.register, other, false, 0, "", nil)
		return err
	})
	if !errors.Is(err, ErrNotShiftOwner) {
		t.Fatalf("pengguna lain harus ErrNotShiftOwner, dapat %v", err)
	}
}
