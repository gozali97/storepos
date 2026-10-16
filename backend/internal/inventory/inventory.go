// Package inventory mengelola saldo stok dan pergerakan stok append-only (CORE-032).
// Stok disimpan dalam milli-unit satuan dasar per (bisnis, cabang, varian).
package inventory

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

type Kind string

const (
	KindReceipt Kind = "RECEIPT"
	KindSale    Kind = "SALE"
)

// ErrNegative: Apply menolak saldo negatif (TBK-04 default: dilarang).
var ErrNegative = errors.New("saldo stok tidak boleh negatif")

type Movement struct {
	BusinessID, BranchID, VariantID idkit.ID
	Kind                            Kind
	QtyMilli                        int64 // bertanda: + masuk, - keluar
	RefType                         string
	RefID                           idkit.ID
	UserID                          idkit.ID
	Reason, Note                    string
	ApprovedBy                      *idkit.ID
}

func ensureRow(ctx context.Context, db database.Getter, biz, branch, variant idkit.ID) error {
	_, err := db.Exec(ctx,
		`INSERT INTO inventory_balances (business_id, branch_id, variant_id) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
		biz, branch, variant)
	return err
}

// Lock mengunci baris saldo (FOR UPDATE) dan mengembalikan saldo saat ini.
// Pemanggil yang mengunci banyak varian WAJIB mengurutkan menurut ID agar tidak deadlock (§13.5).
func Lock(ctx context.Context, db database.Getter, biz, branch, variant idkit.ID) (int64, error) {
	if err := ensureRow(ctx, db, biz, branch, variant); err != nil {
		return 0, fmt.Errorf("inventory: siapkan saldo: %w", err)
	}
	var q int64
	if err := db.QueryRow(ctx,
		`SELECT qty_milli FROM inventory_balances WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3 FOR UPDATE`,
		biz, branch, variant).Scan(&q); err != nil {
		return 0, fmt.Errorf("inventory: kunci saldo: %w", err)
	}
	return q, nil
}

// Apply mengubah saldo dan mencatat pergerakan dengan balance_after.
func Apply(ctx context.Context, db database.Getter, m Movement) (int64, error) {
	if m.QtyMilli == 0 {
		return 0, errors.New("inventory: qty pergerakan tidak boleh nol")
	}
	if err := ensureRow(ctx, db, m.BusinessID, m.BranchID, m.VariantID); err != nil {
		return 0, fmt.Errorf("inventory: siapkan saldo: %w", err)
	}
	var after int64
	if err := db.QueryRow(ctx,
		`UPDATE inventory_balances SET qty_milli = qty_milli + $4, updated_at = now()
		 WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3 AND ($4>=0 OR qty_milli+$4>=reserved_milli) RETURNING qty_milli`,
		m.BusinessID, m.BranchID, m.VariantID, m.QtyMilli).Scan(&after); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNegative
		}
		return 0, fmt.Errorf("inventory: ubah saldo: %w", err)
	}
	if after < 0 {
		return 0, ErrNegative // transaksi pemanggil akan di-rollback
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO stock_movements (id,business_id,branch_id,variant_id,kind,qty_milli,balance_after_milli,ref_type,ref_id,created_by,reason,note,approved_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		idkit.New(), m.BusinessID, m.BranchID, m.VariantID, string(m.Kind), m.QtyMilli, after, m.RefType, m.RefID, m.UserID, m.Reason, m.Note, m.ApprovedBy); err != nil {
		return 0, fmt.Errorf("inventory: catat pergerakan: %w", err)
	}
	return after, nil
}

type ReceiptLine struct {
	VariantID idkit.ID
	QtyMilli  int64
}

// Receive menambah stok dari penerimaan barang.
// shortcut: langsung POSTED tanpa DRAFT, tanpa supplier/HPP; tambah di CORE-029.
func Receive(ctx context.Context, db database.Getter, biz, branch, user idkit.ID, lines []ReceiptLine) (idkit.ID, error) {
	if len(lines) == 0 {
		return idkit.ID{}, errors.New("inventory: penerimaan tanpa baris")
	}
	sorted := append([]ReceiptLine(nil), lines...)
	sort.Slice(sorted, func(i, j int) bool { return bytes.Compare(sorted[i].VariantID[:], sorted[j].VariantID[:]) < 0 })
	ref := idkit.New()
	for _, l := range sorted {
		if l.QtyMilli <= 0 {
			return idkit.ID{}, errors.New("inventory: qty penerimaan harus > 0")
		}
		if _, err := Apply(ctx, db, Movement{BusinessID: biz, BranchID: branch, VariantID: l.VariantID,
			Kind: KindReceipt, QtyMilli: l.QtyMilli, RefType: "receipt", RefID: ref, UserID: user}); err != nil {
			return idkit.ID{}, err
		}
	}
	return ref, nil
}
