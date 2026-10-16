package inventory

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

var ErrCountNotFound = errors.New("sesi opname tidak ditemukan")
var ErrCountConflict = errors.New("versi sesi opname berubah atau sudah diposting")
var ErrCountInvalid = errors.New("item atau jumlah opname tidak valid")

// StartCount takes the complete catalog/balance snapshot in one SQL statement.
// Pharmacy is disabled until per-batch counts are implemented (APT-013).
func StartCount(ctx context.Context, db database.Getter, biz, branch, user idkit.ID) (idkit.ID, error) {
	var valid bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM branches b JOIN businesses t ON t.id=b.business_id WHERE b.business_id=$1 AND b.id=$2 AND b.status='active' AND t.business_type<>'pharmacy')`, biz, branch).Scan(&valid); err != nil {
		return idkit.ID{}, err
	}
	if !valid {
		return idkit.ID{}, ErrCountInvalid
	}
	id := idkit.New()
	if _, err := db.Exec(ctx, `INSERT INTO stock_counts(id,business_id,branch_id,created_by) VALUES($1,$2,$3,$4)`, id, biz, branch, user); err != nil {
		return idkit.ID{}, err
	}
	if _, err := db.Exec(ctx, `INSERT INTO stock_count_lines(business_id,count_id,variant_id,snapshot_qty_milli)
 SELECT v.business_id,$2,v.id,COALESCE(b.qty_milli,0) FROM variants v
 JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id
 LEFT JOIN inventory_balances b ON b.business_id=v.business_id AND b.branch_id=$3 AND b.variant_id=v.id
 WHERE v.business_id=$1 AND p.track_stock AND p.status<>'archived' AND v.status<>'archived'`, biz, id, branch); err != nil {
		return idkit.ID{}, err
	}
	if err := audit.Write(ctx, db, audit.Entry{BusinessID: biz, BranchID: &branch, ActorUserID: user, Action: "stock_count.started", EntityType: "stock_count", EntityID: id.String()}); err != nil {
		return idkit.ID{}, err
	}
	return id, nil
}

func lockCount(ctx context.Context, db database.Getter, biz, id idkit.ID, version int, hasBranch func(idkit.ID) bool) (idkit.ID, error) {
	var branch idkit.ID
	var status string
	var current int
	err := db.QueryRow(ctx, `SELECT branch_id,status,version FROM stock_counts WHERE business_id=$1 AND id=$2 FOR UPDATE`, biz, id).Scan(&branch, &status, &current)
	if errors.Is(err, pgx.ErrNoRows) {
		return branch, ErrCountNotFound
	}
	if err != nil {
		return branch, err
	}
	if !hasBranch(branch) {
		return branch, ErrCountNotFound
	}
	if status != "draft" || current != version {
		return branch, ErrCountConflict
	}
	return branch, nil
}

// RecordCount locks current inventory, not start-of-session inventory. Subsequent sales
// are preserved by posting only the measured discrepancy, not the counted absolute value.
func RecordCount(ctx context.Context, db database.Getter, biz, id, variant, user idkit.ID, version int, physical int64, hasBranch func(idkit.ID) bool) error {
	if physical < 0 {
		return ErrCountInvalid
	}
	branch, err := lockCount(ctx, db, biz, id, version, hasBranch)
	if err != nil {
		return err
	}
	var exists bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stock_count_lines WHERE business_id=$1 AND count_id=$2 AND variant_id=$3)`, biz, id, variant).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrCountInvalid
	}
	system, err := Lock(ctx, db, biz, branch, variant)
	if err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `UPDATE stock_count_lines SET system_qty_at_count=$4,counted_qty_milli=$5,counted_by=$6,counted_at=clock_timestamp() WHERE business_id=$1 AND count_id=$2 AND variant_id=$3`, biz, id, variant, system, physical, user); err != nil {
		return err
	}
	_, err = db.Exec(ctx, `UPDATE stock_counts SET version=version+1 WHERE business_id=$1 AND id=$2`, biz, id)
	return err
}

func PostCount(ctx context.Context, db database.Getter, biz, id, user idkit.ID, version int, hasBranch func(idkit.ID) bool) error {
	branch, err := lockCount(ctx, db, biz, id, version, hasBranch)
	if err != nil {
		return err
	}
	rows, err := db.Query(ctx, `SELECT variant_id,counted_qty_milli-system_qty_at_count FROM stock_count_lines
 WHERE business_id=$1 AND count_id=$2 AND counted_at IS NOT NULL ORDER BY variant_id`, biz, id)
	if err != nil {
		return err
	}
	type line struct {
		id    idkit.ID
		delta int64
	}
	var lines []line
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.id, &l.delta); err != nil {
			rows.Close()
			return err
		}
		lines = append(lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(lines) == 0 {
		return ErrCountInvalid
	}
	for _, l := range lines {
		if l.delta == 0 {
			continue
		}
		if _, err := Apply(ctx, db, Movement{BusinessID: biz, BranchID: branch, VariantID: l.id, Kind: "OPNAME_ADJUSTMENT", QtyMilli: l.delta, RefType: "stock_count", RefID: id, UserID: user, Reason: "stock_count"}); err != nil {
			return err
		}
	}
	if _, err := db.Exec(ctx, `UPDATE stock_counts SET status='posted',version=version+1,posted_at=clock_timestamp(),posted_by=$3 WHERE business_id=$1 AND id=$2`, biz, id, user); err != nil {
		return err
	}
	return audit.Write(ctx, db, audit.Entry{BusinessID: biz, BranchID: &branch, ActorUserID: user, Action: "stock_count.posted", EntityType: "stock_count", EntityID: id.String(), After: map[string]int{"counted_lines": len(lines)}})
}
