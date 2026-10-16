package purchasing

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/inventory"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

var ErrInvalid = errors.New("data penerimaan atau satuan tidak valid")
var ErrNotFound = errors.New("penerimaan tidak ditemukan")
var ErrConflict = errors.New("versi penerimaan berubah atau dokumen sudah diposting")
var ErrUnknownCost = errors.New("saldo awal memiliki HPP belum diketahui; tetapkan HPP awal sebelum posting")

type Line struct {
	UnitID               idkit.ID
	QtyMilli, PriceCents int64
}

// Convert rejects fractional milli-units rather than silently losing inventory.
func Convert(qty, factor int64) (int64, error) {
	if qty <= 0 || factor <= 0 {
		return 0, ErrInvalid
	}
	n := new(big.Int).Mul(big.NewInt(qty), big.NewInt(factor))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, big.NewInt(1000), r)
	if r.Sign() != 0 || !q.IsInt64() || q.Sign() <= 0 {
		return 0, ErrInvalid
	}
	return q.Int64(), nil
}

func roundedRatio(n, den *big.Int) (int64, error) {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, den, r)
	if new(big.Int).Lsh(r, 1).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return 0, ErrInvalid
	}
	return q.Int64(), nil
}

func AverageCost(balance, oldCost, qty, incoming int64) (int64, error) {
	if oldCost < 0 || incoming < 0 || qty <= 0 {
		return 0, ErrInvalid
	}
	if balance <= 0 {
		return incoming, nil
	}
	a := new(big.Int).Mul(big.NewInt(balance), big.NewInt(oldCost))
	a.Add(a, new(big.Int).Mul(big.NewInt(qty), big.NewInt(incoming)))
	den := new(big.Int).Add(big.NewInt(balance), big.NewInt(qty))
	return roundedRatio(a, den)
}

func Create(ctx context.Context, db database.Getter, biz, branch, user idkit.ID, invoice, date string, lines []Line) (idkit.ID, error) {
	if len(lines) == 0 || len(lines) > 500 {
		return idkit.ID{}, ErrInvalid
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return idkit.ID{}, ErrInvalid
	}
	var valid bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM branches b JOIN businesses t ON t.id=b.business_id WHERE b.business_id=$1 AND b.id=$2 AND b.status='active' AND t.business_type<>'pharmacy')`, biz, branch).Scan(&valid); err != nil {
		return idkit.ID{}, err
	}
	if !valid {
		return idkit.ID{}, ErrInvalid
	}
	id := idkit.New()
	if _, err := db.Exec(ctx, `INSERT INTO goods_receipts(id,business_id,branch_id,invoice_number,received_date,created_by) VALUES($1,$2,$3,$4,$5,$6)`, id, biz, branch, strings.TrimSpace(invoice), date, user); err != nil {
		return idkit.ID{}, err
	}
	for _, l := range lines {
		if l.QtyMilli <= 0 || l.PriceCents < 0 {
			return idkit.ID{}, ErrInvalid
		}
		var exists bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM purchase_units WHERE business_id=$1 AND id=$2 AND NOT archived)`, biz, l.UnitID).Scan(&exists); err != nil {
			return idkit.ID{}, err
		}
		if !exists {
			return idkit.ID{}, ErrInvalid
		}
		if _, err := db.Exec(ctx, `INSERT INTO goods_receipt_lines(id,business_id,receipt_id,unit_id,qty_milli,purchase_price_cents) VALUES($1,$2,$3,$4,$5,$6)`, idkit.New(), biz, id, l.UnitID, l.QtyMilli, l.PriceCents); err != nil {
			return idkit.ID{}, err
		}
	}
	return id, nil
}

// Post locks the receipt, unit factors and balances; a second post cannot repeat its effects.
func Post(ctx context.Context, db database.Getter, biz, id, user idkit.ID, version int, hasBranch func(idkit.ID) bool) error {
	var branch idkit.ID
	var status string
	var current int
	err := db.QueryRow(ctx, `SELECT branch_id,status,version FROM goods_receipts WHERE business_id=$1 AND id=$2 FOR UPDATE`, biz, id).Scan(&branch, &status, &current)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !hasBranch(branch) {
		return ErrNotFound
	}
	if status != "draft" || version != current {
		return ErrConflict
	}
	type item struct {
		id, unit, variant              idkit.ID
		name                           string
		qty, price, factor, base, cost int64
	}
	rows, err := db.Query(ctx, `SELECT l.id,l.unit_id,u.variant_id,u.name,l.qty_milli,l.purchase_price_cents,u.factor_milli
 FROM goods_receipt_lines l JOIN purchase_units u ON u.business_id=l.business_id AND u.id=l.unit_id
 JOIN variants v ON v.business_id=u.business_id AND v.id=u.variant_id
 JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id
 WHERE l.business_id=$1 AND l.receipt_id=$2 AND NOT u.archived AND p.track_stock AND v.status='active' AND p.status='active'
 ORDER BY u.id,l.id FOR UPDATE OF u`, biz, id)
	if err != nil {
		return err
	}
	var items []item
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.id, &i.unit, &i.variant, &i.name, &i.qty, &i.price, &i.factor); err != nil {
			rows.Close()
			return err
		}
		items = append(items, i)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	var count int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM goods_receipt_lines WHERE business_id=$1 AND receipt_id=$2`, biz, id).Scan(&count); err != nil {
		return err
	}
	if len(items) != count || count == 0 {
		return ErrInvalid
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].variant == items[j].variant {
			return items[i].id.String() < items[j].id.String()
		}
		return items[i].variant.String() < items[j].variant.String()
	})
	for _, i := range items {
		base, err := Convert(i.qty, i.factor)
		if err != nil {
			return err
		}
		cost, err := roundedRatio(new(big.Int).Mul(big.NewInt(i.price), big.NewInt(1000)), big.NewInt(i.factor))
		if err != nil {
			return err
		}
		balance, err := inventory.Lock(ctx, db, biz, branch, i.variant)
		if err != nil {
			return err
		}
		var old *int64
		if err := db.QueryRow(ctx, `SELECT average_cost_cents FROM inventory_balances WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, biz, branch, i.variant).Scan(&old); err != nil {
			return err
		}
		if balance > 0 && old == nil {
			return ErrUnknownCost
		}
		oldCost := int64(0)
		if old != nil {
			oldCost = *old
		}
		avg, err := AverageCost(balance, oldCost, base, cost)
		if err != nil {
			return err
		}
		if _, err := inventory.Apply(ctx, db, inventory.Movement{BusinessID: biz, BranchID: branch, VariantID: i.variant, Kind: inventory.KindReceipt, QtyMilli: base, RefType: "goods_receipt", RefID: id, UserID: user}); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `UPDATE inventory_balances SET average_cost_cents=$4 WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, biz, branch, i.variant, avg); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `UPDATE goods_receipt_lines SET variant_id=$3,unit_name=$4,factor_milli=$5,base_qty_milli=$6,base_cost_cents=$7 WHERE business_id=$1 AND id=$2`, biz, i.id, i.variant, i.name, i.factor, base, cost); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `UPDATE purchase_units SET used_at=COALESCE(used_at,now()) WHERE business_id=$1 AND id=$2`, biz, i.unit); err != nil {
			return err
		}
	}
	if _, err := db.Exec(ctx, `UPDATE goods_receipts SET status='posted',version=version+1,posted_at=now() WHERE business_id=$1 AND id=$2`, biz, id); err != nil {
		return err
	}
	return audit.Write(ctx, db, audit.Entry{BusinessID: biz, BranchID: &branch, ActorUserID: user, Action: "goods_receipt.posted", EntityType: "goods_receipt", EntityID: id.String()})
}
