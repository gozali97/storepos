package sales

import (
	"context"
	"math/big"

	"github.com/storepos/storepos/internal/inventory"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

// MergeRestockCost uses the total historical return cost, avoiding a rounded unit-price intermediate.
// Unknown positive existing stock or unknown returned cost keeps the result unknown.
func MergeRestockCost(balance int64, old *int64, qty int64, returnedCost *int64) (*int64, error) {
	if balance < 0 || qty <= 0 {
		return nil, ErrReturnQuantity
	}
	if returnedCost == nil || (balance > 0 && old == nil) {
		return nil, nil
	}
	if *returnedCost < 0 || (old != nil && *old < 0) {
		return nil, ErrReturnQuantity
	}
	n := new(big.Int).Mul(big.NewInt(*returnedCost), big.NewInt(1000))
	if balance > 0 {
		n.Add(n, new(big.Int).Mul(big.NewInt(balance), big.NewInt(*old)))
	}
	den := new(big.Int).Add(big.NewInt(balance), big.NewInt(qty))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, den, r)
	if new(big.Int).Lsh(r, 1).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return nil, ErrReturnQuantity
	}
	v := q.Int64()
	return &v, nil
}
func restockWithCost(ctx context.Context, db database.Getter, biz, branch, variant, user, ref idkit.ID, qty int64, cost *int64) error {
	balance, err := inventory.Lock(ctx, db, biz, branch, variant)
	if err != nil {
		return err
	}
	var old *int64
	if err := db.QueryRow(ctx, `SELECT average_cost_cents FROM inventory_balances WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, biz, branch, variant).Scan(&old); err != nil {
		return err
	}
	avg, err := MergeRestockCost(balance, old, qty, cost)
	if err != nil {
		return err
	}
	if _, err := inventory.Apply(ctx, db, inventory.Movement{BusinessID: biz, BranchID: branch, VariantID: variant, Kind: "RETURN_RESTOCK", QtyMilli: qty, RefType: "return", RefID: ref, UserID: user}); err != nil {
		return err
	}
	_, err = db.Exec(ctx, `UPDATE inventory_balances SET average_cost_cents=$4 WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, biz, branch, variant, avg)
	return err
}
