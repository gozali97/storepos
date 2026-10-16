package sales

import (
	"context"
	"encoding/json"
	"testing"
)

func TestReceiptHeaderDoesNotFollowMasterChanges(t *testing.T) {
	e := setup(t, 5)
	ctx := context.Background()
	db := e.pool.Underlying()
	if _, err := db.Exec(ctx, `UPDATE branches SET receipt_header='Header asli',receipt_footer='Footer asli',address='Alamat asli'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE users SET display_name='Kasir asli' WHERE id=$1`, e.user); err != nil {
		t.Fatal(err)
	}
	r, err := e.sell(1, 1500000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE businesses SET name='Bisnis berubah'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE branches SET name='Cabang berubah',receipt_header='Header berubah',receipt_footer='Footer berubah'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE users SET display_name='Nama berubah' WHERE id=$1`, e.user); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := db.QueryRow(ctx, `SELECT receipt_header_snapshot FROM sales WHERE id=$1`, r.SaleID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if p["business_name"] != "T" || p["branch_name"] != "Pusat" || p["receipt_header"] != "Header asli" || p["receipt_footer"] != "Footer asli" || p["timezone"] != "Asia/Jakarta" || p["cashier_name"] != "Kasir asli" {
		t.Fatal(p)
	}
}
