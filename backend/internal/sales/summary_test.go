package sales

import (
	"context"
	"errors"
	"github.com/storepos/storepos/internal/platform/idkit"
	"testing"
)

func TestClosedShiftSummaryAndIsolation(t *testing.T) {
	e := setup(t, 5)
	ctx := context.Background()
	db := e.pool.Underlying()
	var sid idkit.ID
	if err := db.QueryRow(ctx, `SELECT id FROM shifts`).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	allow := func(idkit.ID) bool { return true }
	if _, err := Summary(ctx, db, e.biz, sid, e.user, false, allow); !errors.Is(err, ErrShiftStillOpen) {
		t.Fatalf("blind count: %v", err)
	}
	if _, err := e.sell(1, 2000000); err != nil {
		t.Fatal(err)
	}
	if _, err := e.closeShift(false, 1500000, "", nil); err != nil {
		t.Fatal(err)
	}
	r, err := Summary(ctx, db, e.biz, sid, e.user, false, allow)
	if err != nil {
		t.Fatal(err)
	}
	if r.Expected != "1500000" || r.Variance != "0" || r.SaleCount != 1 || len(r.Payments) != 1 || r.Payments[0].Change != "500000" {
		t.Fatalf("reconciliation: %+v", r)
	}
	if _, err := Summary(ctx, db, idkit.New(), sid, e.user, true, allow); !errors.Is(err, ErrNotFound) {
		t.Fatalf("tenant: %v", err)
	}
	if _, err := Summary(ctx, db, e.biz, sid, idkit.New(), false, allow); !errors.Is(err, ErrNotShiftOwner) {
		t.Fatalf("owner: %v", err)
	}
	if _, err := Summary(ctx, db, e.biz, sid, e.user, true, func(idkit.ID) bool { return false }); !errors.Is(err, ErrNotShiftOwner) {
		t.Fatalf("branch: %v", err)
	}
}
