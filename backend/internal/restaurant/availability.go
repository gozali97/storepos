package restaurant

import (
	"context"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

func SoldOut(ctx context.Context, db database.Getter, biz, branch, variant idkit.ID) (bool, error) {
	var soldOut bool
	err := db.QueryRow(ctx, `SELECT COALESCE((SELECT sold_out FROM restaurant_menu_availability WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3),false)`, biz, branch, variant).Scan(&soldOut)
	return soldOut, err
}
