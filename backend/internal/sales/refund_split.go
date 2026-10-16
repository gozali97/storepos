package sales

import (
	"context"
	"sort"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

// RefundPart adalah bagian refund yang dikembalikan lewat satu metode pembayaran asal.
type RefundPart struct {
	Method string `json:"method"`
	Amount int64  `json:"-"`
	Cents  string `json:"amount_cents"`
	Manual bool   `json:"manual_refund_required"`
}

type methodLeft struct {
	Method string
	Left   int64 // dibayar − sudah direfund lewat metode ini
}

// AllocateRefund membagi refund ke metode asal: nontunai (urut nama) dulu, tunai terakhir,
// masing-masing dibatasi sisa yang belum direfund. Aturan ini keputusan produk [REK]; ubah di sini bila berbeda.
func AllocateRefund(total int64, left []methodLeft) ([]RefundPart, error) {
	order := append([]methodLeft(nil), left...)
	sort.SliceStable(order, func(i, j int) bool {
		if (order[i].Method == "cash") != (order[j].Method == "cash") {
			return order[j].Method == "cash"
		}
		return order[i].Method < order[j].Method
	})
	var parts []RefundPart
	for _, m := range order {
		if total == 0 {
			break
		}
		take := m.Left
		if take > total {
			take = total
		}
		if take <= 0 {
			continue
		}
		parts = append(parts, RefundPart{Method: m.Method, Amount: take, Manual: m.Method != "cash"})
		total -= take
	}
	if total != 0 {
		return nil, ErrReturnQuantity // refund melebihi sisa pembayaran yang dapat dikembalikan
	}
	return parts, nil
}

// refundable membaca sisa per metode; panggil setelah baris transaksi terkunci agar tidak berlomba.
func refundable(ctx context.Context, db database.Getter, biz, sale idkit.ID) ([]methodLeft, error) {
	rows, err := db.Query(ctx, `SELECT p.method,(p.paid-COALESCE(f.refunded,0))::bigint FROM
 (SELECT method,SUM(amount_cents)::bigint paid FROM payments WHERE business_id=$1 AND sale_id=$2 GROUP BY method) p
 LEFT JOIN (SELECT f.method,SUM(f.amount_cents)::bigint refunded FROM refunds f JOIN sale_returns r ON r.business_id=f.business_id AND r.id=f.return_id
  WHERE r.business_id=$1 AND r.sale_id=$2 GROUP BY f.method) f ON f.method=p.method`, biz, sale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []methodLeft
	for rows.Next() {
		var m methodLeft
		if err := rows.Scan(&m.Method, &m.Left); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
