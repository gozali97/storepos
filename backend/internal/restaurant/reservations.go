package restaurant

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/inventory"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

var ErrReservedStock = errors.New("stok tersedia tidak mencukupi untuk reservasi menu")

// Caller sorts variant IDs before reserving multiple lines, using the same order as sales.
func Reserve(ctx context.Context, db database.Getter, biz, branch, line, variant idkit.ID, qty int64) error {
	if qty <= 0 {
		return ErrReservedStock
	}
	onHand, err := inventory.Lock(ctx, db, biz, branch, variant)
	if err != nil {
		return err
	}
	var reserved int64
	if err := db.QueryRow(ctx, `SELECT reserved_milli FROM inventory_balances WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, biz, branch, variant).Scan(&reserved); err != nil {
		return err
	}
	if onHand < reserved || qty > onHand-reserved {
		return ErrReservedStock
	}
	if _, err := db.Exec(ctx, `INSERT INTO restaurant_stock_reservations(business_id,branch_id,line_id,variant_id,qty_milli) VALUES($1,$2,$3,$4,$5)`, biz, branch, line, variant, qty); err != nil {
		return err
	}
	_, err = db.Exec(ctx, `UPDATE inventory_balances SET reserved_milli=reserved_milli+$4 WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, biz, branch, variant, qty)
	return err
}
func Release(ctx context.Context, db database.Getter, biz, line idkit.ID) error {
	var branch, variant idkit.ID
	var qty int64
	var status string
	err := db.QueryRow(ctx, `SELECT branch_id,variant_id,qty_milli,status FROM restaurant_stock_reservations WHERE business_id=$1 AND line_id=$2 FOR UPDATE`, biz, line).Scan(&branch, &variant, &qty, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if status != "active" {
		return nil
	}
	if _, err := inventory.Lock(ctx, db, biz, branch, variant); err != nil {
		return err
	}
	tag, err := db.Exec(ctx, `UPDATE inventory_balances SET reserved_milli=reserved_milli-$4 WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3 AND reserved_milli >= $4`, biz, branch, variant, qty)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("saldo reservasi tidak konsisten")
	}
	_, err = db.Exec(ctx, `UPDATE restaurant_stock_reservations SET status='released' WHERE business_id=$1 AND line_id=$2`, biz, line)
	return err
}
