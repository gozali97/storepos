package sales

import (
	"context"
	"errors"
	"github.com/storepos/storepos/internal/platform/database"
	"strings"
	"testing"
)

func TestFinalNotesSnapshotAndValidation(t *testing.T) {
	e := setup(t, 5)
	ctx := context.Background()
	run := func(note, lineNote string) (*Result, error) {
		var out *Result
		err := e.inTx(func(ctx context.Context, tx database.Tx) error {
			var err error
			out, err = Finalize(ctx, tx, Input{BusinessID: e.biz, BranchID: e.branch, RegisterID: e.register, UserID: e.user, Note: note, Lines: []LineIn{{VariantID: e.variant, QtyMilli: 1000, Note: lineNote}}, Payments: []PaymentIn{{Method: "cash", AmountCents: 1500000}}})
			return err
		})
		return out, err
	}
	if _, err := run(strings.Repeat("é", 201), ""); !errors.Is(err, ErrInvalidNote) {
		t.Fatal(err)
	}
	if _, err := run("", strings.Repeat("a", 201)); !errors.Is(err, ErrInvalidNote) {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM sales`); n != 0 {
		t.Fatal("invalid notes saved sale")
	}
	out, err := run("Catatan transaksi", strings.Repeat("é", 200))
	if err != nil {
		t.Fatal(err)
	}
	var header, line string
	if err := e.pool.Underlying().QueryRow(ctx, `SELECT s.note,l.note FROM sales s JOIN sale_lines l ON l.sale_id=s.id WHERE s.id=$1`, out.SaleID).Scan(&header, &line); err != nil {
		t.Fatal(err)
	}
	if header != "Catatan transaksi" || line != strings.Repeat("é", 200) {
		t.Fatal(header, line)
	}
}
