package sales

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/storepos/storepos/internal/pharmacy"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

var (
	ErrPrescriptionRequired = errors.New("produk wajib referensi resep dan verifikasi apoteker")
	ErrBatchOverride        = errors.New("override batch hanya oleh apoteker, wajib alasan, ke batch sellable yang cukup")
)

type pharmLine struct {
	idx      int
	variant  idkit.ID
	qty      int64
	override *idkit.ID
	reason   string
}

type batchAlloc struct {
	batch          idkit.ID
	number, expiry string
	qty            int64
	reason         string
}

// checkPrescription: produk wajib resep butuh referensi + verifikasi apoteker (APT-008, APT-009).
func checkPrescription(ctx context.Context, db database.Getter, in Input, infos []vinfo, lines []pharmLine) error {
	ref := strings.TrimSpace(in.PrescriptionRef)
	if !utf8.ValidString(ref) || utf8.RuneCountInString(ref) > 200 {
		return ErrInvalidNote
	}
	for _, l := range lines {
		var required bool
		err := db.QueryRow(ctx, `SELECT COALESCE((SELECT prescription_required FROM pharmacy_drugs WHERE business_id=$1 AND product_id=$2),false)`, in.BusinessID, infos[l.idx].ProductID).Scan(&required)
		if err != nil {
			return err
		}
		if required && (ref == "" || !in.PharmacistVerified) {
			return ErrPrescriptionRequired
		}
	}
	return nil
}

// allocateBatches mengunci baris saldo batch (urut varian → ED → ID, tanpa deadlock) lalu
// membagi kuantitas secara FEFO; override manual harus menutupi seluruh baris (APT-005).
func allocateBatches(ctx context.Context, db database.Getter, in Input, today string, infos []vinfo, lines []pharmLine) (map[int][]batchAlloc, error) {
	byVariant := map[idkit.ID][]pharmLine{}
	ids := []idkit.ID{}
	for _, l := range lines {
		if _, ok := byVariant[l.variant]; !ok {
			ids = append(ids, l.variant)
		}
		byVariant[l.variant] = append(byVariant[l.variant], l)
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	type avail struct {
		id             idkit.ID
		number, expiry string
		qty            int64
	}
	out := map[int][]batchAlloc{}
	var short []Shortage
	for _, variant := range ids {
		rows, err := db.Query(ctx, `SELECT b.id,b.batch_number,b.expiry_date::text,x.qty_milli
 FROM pharmacy_batches b JOIN pharmacy_batch_balances x ON x.business_id=b.business_id AND x.batch_id=b.id
 WHERE b.business_id=$1 AND x.branch_id=$2 AND b.variant_id=$3 AND x.status='SELLABLE' AND x.qty_milli>0 AND b.expiry_date>$4::date
 ORDER BY b.expiry_date,b.id FOR UPDATE OF x`, in.BusinessID, in.BranchID, variant, today)
		if err != nil {
			return nil, err
		}
		var pool []avail
		for rows.Next() {
			var a avail
			if err := rows.Scan(&a.id, &a.number, &a.expiry, &a.qty); err != nil {
				rows.Close()
				return nil, err
			}
			pool = append(pool, a)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		for _, l := range byVariant[variant] {
			need := l.qty
			if l.override != nil {
				found := false
				for i := range pool {
					if pool[i].id != *l.override {
						continue
					}
					found = true
					if pool[i].qty < need {
						return nil, ErrBatchOverride
					}
					pool[i].qty -= need
					out[l.idx] = []batchAlloc{{pool[i].id, pool[i].number, pool[i].expiry, need, l.reason}}
					need = 0
				}
				if !found {
					var expired bool
					err := db.QueryRow(ctx, `SELECT COALESCE((SELECT expiry_date<=$3::date FROM pharmacy_batches WHERE business_id=$1 AND id=$2 AND variant_id=$4),false)`, in.BusinessID, *l.override, today, variant).Scan(&expired)
					if err != nil {
						return nil, err
					}
					if expired {
						return nil, pharmacy.ErrExpired
					}
					return nil, ErrBatchOverride
				}
				continue
			}
			var total int64
			for i := range pool {
				total += pool[i].qty
				if need == 0 || pool[i].qty == 0 {
					continue
				}
				take := pool[i].qty
				if take > need {
					take = need
				}
				pool[i].qty -= take
				need -= take
				out[l.idx] = append(out[l.idx], batchAlloc{pool[i].id, pool[i].number, pool[i].expiry, take, ""})
			}
			if need > 0 {
				v := infos[l.idx]
				short = append(short, Shortage{variant.String(), v.SKU, v.ProductName, l.qty, total})
			}
		}
	}
	if len(short) > 0 {
		return nil, &InsufficientError{Items: short}
	}
	return out, nil
}

// recordBatchSale menyimpan snapshot alokasi dan mengurangi saldo batch dalam transaksi penjualan.
func recordBatchSale(ctx context.Context, db database.Getter, in Input, saleID, lineID idkit.ID, allocs []batchAlloc) error {
	for _, a := range allocs {
		if err := pharmacy.Move(ctx, db, in.BusinessID, in.BranchID, a.batch, in.UserID, saleID, "SELLABLE", "SALE", "", -a.qty); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO sale_line_batches(business_id,sale_id,sale_line_id,batch_id,branch_id,batch_number,expiry_date,qty_milli,override_reason) VALUES($1,$2,$3,$4,$5,$6,$7::date,$8,$9)`,
			in.BusinessID, saleID, lineID, a.batch, in.BranchID, a.number, a.expiry, a.qty, a.reason); err != nil {
			return err
		}
	}
	return nil
}

// restoreBatchesForVoid mengembalikan seluruh alokasi ke batch asal sebagai SELLABLE.
func restoreBatchesForVoid(ctx context.Context, db database.Getter, biz, branch, sale, user idkit.ID) error {
	rows, err := db.Query(ctx, `SELECT batch_id,SUM(qty_milli-returned_milli)::bigint FROM sale_line_batches WHERE business_id=$1 AND sale_id=$2 GROUP BY batch_id HAVING SUM(qty_milli-returned_milli)>0 ORDER BY batch_id`, biz, sale)
	if err != nil {
		return err
	}
	type item struct {
		batch idkit.ID
		qty   int64
	}
	var items []item
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.batch, &i.qty); err != nil {
			rows.Close()
			return err
		}
		items = append(items, i)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, i := range items {
		if err := pharmacy.Move(ctx, db, biz, branch, i.batch, user, sale, "SELLABLE", "SALE_VOID", "", i.qty); err != nil {
			return err
		}
	}
	return nil
}

// returnToBatches memakai alokasi penjualan asli (ED terdekat dulu). Retur "restock" masuk
// QUARANTINE di batch asal sampai apoteker melepasnya (APT-010); "damaged" hanya menutup alokasi.
func returnToBatches(ctx context.Context, db database.Getter, biz, branch, line, user, ref idkit.ID, qty int64, quarantine bool) error {
	rows, err := db.Query(ctx, `SELECT batch_id,qty_milli-returned_milli FROM sale_line_batches WHERE business_id=$1 AND sale_line_id=$2 AND qty_milli>returned_milli ORDER BY expiry_date,batch_id FOR UPDATE`, biz, line)
	if err != nil {
		return err
	}
	type item struct {
		batch idkit.ID
		left  int64
	}
	var items []item
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.batch, &i.left); err != nil {
			rows.Close()
			return err
		}
		items = append(items, i)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, i := range items {
		if qty == 0 {
			break
		}
		take := i.left
		if take > qty {
			take = qty
		}
		qty -= take
		if _, err := db.Exec(ctx, `UPDATE sale_line_batches SET returned_milli=returned_milli+$4 WHERE business_id=$1 AND sale_line_id=$2 AND batch_id=$3`, biz, line, i.batch, take); err != nil {
			return err
		}
		if quarantine {
			if err := pharmacy.Move(ctx, db, biz, branch, i.batch, user, ref, "QUARANTINE", "RETURN_QUARANTINE", "", take); err != nil {
				return err
			}
		}
	}
	if qty != 0 {
		return ErrReturnQuantity
	}
	return nil
}
