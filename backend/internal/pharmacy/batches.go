package pharmacy

import (
	"context"
	"errors"
	"time"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

var ErrBatch = errors.New("batch/status/kuantitas tidak valid atau stok tidak cukup")
var ErrExpired = errors.New("batch kedaluwarsa tidak boleh menjadi stok jual")

func Expiry(s string) (time.Time, error) {
	if len(s) == 7 {
		t, err := time.Parse("2006-01", s)
		if err != nil {
			return t, err
		}
		return t.AddDate(0, 1, -1), nil
	}
	return time.Parse("2006-01-02", s)
}
func BranchDate(ctx context.Context, db database.Getter, biz, branch idkit.ID) (time.Time, error) {
	var date string
	err := db.QueryRow(ctx, `SELECT (clock_timestamp() AT TIME ZONE timezone)::date::text FROM branches WHERE business_id=$1 AND id=$2 AND status='active'`, biz, branch).Scan(&date)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse("2006-01-02", date)
}

// Move must be called in a transaction. Negative deltas are checked by the UPDATE itself.
func Move(ctx context.Context, db database.Getter, biz, branch, batch, user, ref idkit.ID, status, kind, reason string, qty int64) error {
	if qty == 0 || (status != "SELLABLE" && status != "QUARANTINE") {
		return ErrBatch
	}
	if _, err := db.Exec(ctx, `INSERT INTO pharmacy_batch_balances(business_id,branch_id,batch_id,status,qty_milli) VALUES($1,$2,$3,$4,0) ON CONFLICT DO NOTHING`, biz, branch, batch, status); err != nil {
		return err
	}
	var after int64
	tag, err := db.Exec(ctx, `UPDATE pharmacy_batch_balances SET qty_milli=qty_milli+$5 WHERE business_id=$1 AND branch_id=$2 AND batch_id=$3 AND status=$4 AND ($5>=0 OR qty_milli>= -$5)`, biz, branch, batch, status, qty)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrBatch
	}
	if err := db.QueryRow(ctx, `SELECT qty_milli FROM pharmacy_batch_balances WHERE business_id=$1 AND branch_id=$2 AND batch_id=$3 AND status=$4`, biz, branch, batch, status).Scan(&after); err != nil {
		return err
	}
	_, err = db.Exec(ctx, `INSERT INTO pharmacy_batch_movements(id,business_id,branch_id,batch_id,status,kind,qty_milli,balance_after_milli,ref_id,reason,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, idkit.New(), biz, branch, batch, status, kind, qty, after, ref, reason, user)
	return err
}
