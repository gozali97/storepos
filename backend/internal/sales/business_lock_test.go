package sales

import (
	"context"
	"github.com/storepos/storepos/internal/org"
	"testing"
)

func TestFirstSaleTypeLockRollsBackOnFailure(t *testing.T) {
	e := setup(t, 1)
	ctx := context.Background()
	db := e.pool.Underlying()
	if _, err := e.sell(2, 5000000); err == nil {
		t.Fatal("oversell accepted")
	}
	var locked bool
	if err := db.QueryRow(ctx, `SELECT first_sale_completed_at IS NOT NULL FROM businesses WHERE id=$1`, e.biz).Scan(&locked); err != nil || locked {
		t.Fatalf("failed sale locked type: %v %v", locked, err)
	}
	if err := org.NewRepo(db).UpdateBusinessType(ctx, e.biz, org.TypeRestaurant); err != nil {
		t.Fatal(err)
	}
	if _, err := e.sell(1, 1500000); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SELECT first_sale_completed_at IS NOT NULL FROM businesses WHERE id=$1`, e.biz).Scan(&locked); err != nil || !locked {
		t.Fatalf("successful sale missing lock: %v %v", locked, err)
	}
	if err := org.NewRepo(db).UpdateBusinessType(ctx, e.biz, org.TypeRetail); err != org.ErrTypeLocked {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE businesses SET business_type='retail' WHERE id=$1`, e.biz); err == nil {
		t.Fatal("DB type guard bypassed")
	}
}
