package sales

import (
	"context"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/reporting"
	"testing"
	"time"
)

func TestSalesReportTaxReturnsAndHistoricalMissing(t *testing.T) {
	for _, mode := range []string{"exclusive", "inclusive"} {
		t.Run(mode, func(t *testing.T) {
			e := setup(t, 5)
			ctx := context.Background()
			db := e.pool.Underlying()
			if err := e.inTx(func(ctx context.Context, tx database.Tx) error {
				return SetTaxRule(ctx, tx, e.biz, e.user, "Demo", 1000, mode)
			}); err != nil {
				t.Fatal(err)
			}
			sale, err := e.sell(2, 4000000)
			if err != nil {
				t.Fatal(err)
			}
			var line idkit.ID
			if err := db.QueryRow(ctx, `SELECT id FROM sale_lines WHERE sale_id=$1`, sale.SaleID).Scan(&line); err != nil {
				t.Fatal(err)
			}
			err = e.inTx(func(ctx context.Context, tx database.Tx) error {
				_, err := Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: sale.SaleID, UserID: e.user, RegisterID: e.register, Reason: "test", Lines: []ReturnLine{{LineID: line, Qty: 1000, Condition: "restock"}}, HasBranch: func(idkit.ID) bool { return true }})
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(ctx, `UPDATE sales SET completed_at='2026-10-12T17:30:00Z'`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(ctx, `UPDATE sale_returns SET created_at='2026-10-13T17:30:00Z'`); err != nil {
				t.Fatal(err)
			}
			from := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
			f := reporting.PaymentFilter{BusinessID: e.biz, BranchID: e.branch, From: from, Until: from.AddDate(0, 0, 4), Timezone: "Asia/Jakarta"}
			rows, err := reporting.Sales(ctx, db, f)
			if err != nil || len(rows) != 2 {
				t.Fatalf("report: %+v %v", rows, err)
			}
			if rows[0].Date != "2026-10-13" || rows[1].Date != "2026-10-14" || rows[0].SaleCount != 1 || rows[1].SaleCount != 0 {
				t.Fatal(rows)
			}
			if mode == "exclusive" {
				if rows[0].Gross != "3000000" || *rows[0].Tax != "300000" || *rows[1].Returns != "1500000" || *rows[1].Tax != "-150000" {
					t.Fatal(rows)
				}
			} else {
				if rows[0].Gross != "2727300" || *rows[0].Tax != "272700" || *rows[1].Returns != "1363600" || *rows[1].Tax != "-136400" {
					t.Fatal(rows)
				}
			}
			if _, err := db.Exec(ctx, `UPDATE sale_return_lines SET tax_cents=NULL`); err != nil {
				t.Fatal(err)
			}
			rows, err = reporting.Sales(ctx, db, f)
			if err != nil {
				t.Fatal(err)
			}
			if rows[1].IncompleteReturns != 1 || rows[1].Net != nil || rows[1].Tax != nil {
				t.Fatal("missing history presented as complete", rows)
			}
		})
	}
}
