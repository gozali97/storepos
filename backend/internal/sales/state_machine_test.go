package sales

import (
	"context"
	"strings"
	"testing"

	"github.com/storepos/storepos/internal/platform/idkit"
)

// PRD §13.1/§13.2: transisi status di luar tabel ditolak database dengan SQLSTATE ST001, apa pun jalur tulisnya.
func TestDatabaseRejectsInvalidStatusTransitions(t *testing.T) {
	e := setup(t, 10)
	r, err := e.sell(1, 1500000)
	if err != nil {
		t.Fatal(err)
	}
	db := e.pool.Underlying()
	ctx := context.Background()
	expectRejected := func(name, sql string, args ...any) {
		t.Helper()
		_, err := db.Exec(ctx, sql, args...)
		if err == nil || !strings.Contains(err.Error(), "ST001") {
			t.Errorf("%s: seharusnya ditolak dengan ST001, hasil: %v", name, err)
		}
	}
	// Transaksi COMPLETED hanya boleh menjadi VOIDED, dan VOIDED tidak boleh kembali.
	if _, err := db.Exec(ctx, `UPDATE sales SET status='voided' WHERE id=$1`, r.SaleID); err != nil {
		t.Fatalf("completed -> voided harus sah: %v", err)
	}
	expectRejected("voided -> completed", `UPDATE sales SET status='completed' WHERE id=$1`, r.SaleID)

	// Pembayaran: confirmed -> reversed saja.
	var pay idkit.ID
	if err := db.QueryRow(ctx, `SELECT id FROM payments WHERE sale_id=$1`, r.SaleID).Scan(&pay); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE payments SET status='reversed' WHERE id=$1`, pay); err != nil {
		t.Fatalf("confirmed -> reversed harus sah: %v", err)
	}
	expectRejected("reversed -> confirmed", `UPDATE payments SET status='confirmed' WHERE id=$1`, pay)

	// Status retur hanya maju.
	r2, err := e.sell(1, 1500000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE sales SET return_status='partial' WHERE id=$1`, r2.SaleID); err != nil {
		t.Fatalf("none -> partial harus sah: %v", err)
	}
	expectRejected("partial -> none", `UPDATE sales SET return_status='none' WHERE id=$1`, r2.SaleID)
	if _, err := db.Exec(ctx, `UPDATE sales SET return_status='full' WHERE id=$1`, r2.SaleID); err != nil {
		t.Fatalf("partial -> full harus sah: %v", err)
	}
	expectRejected("full -> partial", `UPDATE sales SET return_status='partial' WHERE id=$1`, r2.SaleID)

	// Draf: selesai/dibatalkan bersifat final.
	var biz, branch, reg = e.biz, e.branch, e.register
	draft := idkit.New()
	if _, err := db.Exec(ctx, `INSERT INTO sale_drafts(id,business_id,branch_id,register_id,status,payload,created_by) VALUES($1,$2,$3,$4,'held','{}',$5)`, draft, biz, branch, reg, e.user); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE sale_drafts SET status='cancelled' WHERE id=$1`, draft); err != nil {
		t.Fatalf("held -> cancelled harus sah: %v", err)
	}
	expectRejected("cancelled -> active", `UPDATE sale_drafts SET status='active' WHERE id=$1`, draft)
}
