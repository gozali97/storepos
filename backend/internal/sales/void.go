package sales

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/inventory"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

var ErrVoidClosedShift = errors.New("shift asal sudah ditutup; gunakan retur")
var ErrVoidState = errors.New("hanya transaksi completed yang dapat di-void")

// Void must run in the caller's transaction. Restore original SALE movements,
// not current track_stock settings, which may have changed since the sale.
func Void(ctx context.Context, db database.Getter, biz, sale, user, approver idkit.ID, reason string, hasBranch func(idkit.ID) bool) error {
	reason = strings.TrimSpace(reason)
	if reason == "" || utf8.RuneCountInString(reason) > 200 {
		return ErrReasonRequired
	}
	if approver.IsZero() || approver == user {
		return ErrApprovalRequired
	}
	var branch, register, shift idkit.ID
	err := db.QueryRow(ctx, `SELECT branch_id,register_id,shift_id FROM sales WHERE business_id=$1 AND id=$2`, biz, sale).Scan(&branch, &register, &shift)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !hasBranch(branch) {
		return ErrNotShiftOwner
	}
	// Same lock order as finalize: shift, then sale/balances. Closing waits for reversal.
	var shiftStatus string
	if err := db.QueryRow(ctx, `SELECT status FROM shifts WHERE business_id=$1 AND id=$2 FOR SHARE`, biz, shift).Scan(&shiftStatus); err != nil {
		return err
	}
	if shiftStatus != "open" {
		return ErrVoidClosedShift
	}
	var status, returnStatus string
	if err := db.QueryRow(ctx, `SELECT status,return_status FROM sales WHERE business_id=$1 AND id=$2 FOR UPDATE`, biz, sale).Scan(&status, &returnStatus); err != nil {
		return err
	}
	if status != "completed" || returnStatus != "none" {
		return ErrVoidState
	}
	rows, err := db.Query(ctx, `SELECT variant_id,-SUM(qty_milli)::bigint FROM stock_movements
 WHERE business_id=$1 AND branch_id=$2 AND ref_type='sale' AND ref_id=$3 AND kind='SALE'
 GROUP BY variant_id ORDER BY variant_id`, biz, branch, sale)
	if err != nil {
		return err
	}
	type item struct {
		id  idkit.ID
		qty int64
	}
	var items []item
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.id, &i.qty); err != nil {
			rows.Close()
			return err
		}
		items = append(items, i)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	var pharmacyBiz bool
	if err := db.QueryRow(ctx, `SELECT business_type='pharmacy' FROM businesses WHERE id=$1`, biz).Scan(&pharmacyBiz); err != nil {
		return err
	}
	if pharmacyBiz {
		if err := restoreBatchesForVoid(ctx, db, biz, branch, sale, user); err != nil {
			return err
		}
	}
	for _, i := range items {
		if _, err := inventory.Apply(ctx, db, inventory.Movement{BusinessID: biz, BranchID: branch, VariantID: i.id, Kind: inventory.Kind("SALE_VOID"), QtyMilli: i.qty, RefType: "sale_void", RefID: sale, UserID: user}); err != nil {
			return err
		}
	}
	if _, err := db.Exec(ctx, `UPDATE payments SET status='reversed',manual_refund_required=(method<>'cash') WHERE business_id=$1 AND sale_id=$2`, biz, sale); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `UPDATE sales SET status='voided',voided_at=now(),voided_by=$3,void_approved_by=$4,void_reason=$5 WHERE business_id=$1 AND id=$2`, biz, sale, user, approver, reason); err != nil {
		return err
	}
	return audit.Write(ctx, db, audit.Entry{BusinessID: biz, BranchID: &branch, RegisterID: &register, ActorUserID: user, ApprovedBy: &approver, Action: "sale.voided", EntityType: "sale", EntityID: sale.String(), Before: map[string]string{"status": "completed"}, After: map[string]string{"status": "voided"}, Reason: reason})
}
