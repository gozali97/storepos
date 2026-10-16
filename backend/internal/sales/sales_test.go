package sales

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/storepos/storepos/internal/catalog"
	"github.com/storepos/storepos/internal/inventory"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
)

type env struct {
	pool                        *database.Pool
	biz, branch, register, user idkit.ID
	variant                     idkit.ID
}

// setup membuat bisnis, cabang, terminal, shift terbuka, dan satu produk berharga Rp15.000 dengan stok `stock` unit.
func setup(t *testing.T, stock int64) *env {
	t.Helper()
	ctx := context.Background()
	e := &env{pool: testdb.New(t), biz: idkit.New(), branch: idkit.New(), register: idkit.New(), user: idkit.New()}
	db := e.pool.Underlying()
	must := func(_ any, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.Exec(ctx, `INSERT INTO users (id,email,password_hash) VALUES ($1,'k@x.test','x')`, e.user))
	must(db.Exec(ctx, `INSERT INTO businesses (id,name,business_type,enabled_modules,created_by) VALUES ($1,'T','retail',ARRAY['retail'],$2)`, e.biz, e.user))
	must(db.Exec(ctx, `INSERT INTO branches (id,business_id,code,name) VALUES ($1,$2,'JKT','Pusat')`, e.branch, e.biz))
	must(db.Exec(ctx, `INSERT INTO registers (id,business_id,branch_id,name) VALUES ($1,$2,$3,'K1')`, e.register, e.biz, e.branch))
	p := &catalog.Product{Name: "Kopi", TrackStock: true}
	if err := catalog.NewRepo(db).CreateProduct(ctx, e.biz, p, []catalog.NewVariant{{SKU: "K-1", PriceCents: 1500000}}); err != nil {
		t.Fatal(err)
	}
	e.variant = p.Variants[0].ID
	if _, err := OpenShift(ctx, db, e.biz, e.branch, e.register, e.user, 0); err != nil {
		t.Fatal(err)
	}
	if stock > 0 {
		if _, err := inventory.Receive(ctx, db, e.biz, e.branch, e.user, []inventory.ReceiptLine{{VariantID: e.variant, QtyMilli: stock * 1000}}); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func (e *env) sell(qtyUnits, cashCents int64) (*Result, error) {
	var res *Result
	err := e.pool.InTx(context.Background(), func(ctx context.Context, tx database.Tx) error {
		r, err := Finalize(ctx, tx, Input{BusinessID: e.biz, BranchID: e.branch, RegisterID: e.register, UserID: e.user,
			Lines:    []LineIn{{VariantID: e.variant, QtyMilli: qtyUnits * 1000}},
			Payments: []PaymentIn{{Method: "cash", AmountCents: cashCents}}})
		res = r
		return err
	})
	return res, err
}

func (e *env) count(q string) (n int) {
	_ = e.pool.Underlying().QueryRow(context.Background(), q).Scan(&n)
	return
}

func TestFinalizeHappyPath(t *testing.T) {
	e := setup(t, 10)
	r, err := e.sell(2, 5000000) // total Rp30.000, bayar Rp50.000
	if err != nil {
		t.Fatal(err)
	}
	if r.TotalCents != 3000000 || r.ChangeCents != 2000000 {
		t.Fatalf("total/kembalian salah: %+v", r)
	}
	if r.ReceiptNumber == "" || r.ReceiptNumber[:4] != "JKT-" {
		t.Fatalf("nomor struk salah: %q", r.ReceiptNumber)
	}
	if got := e.count(`SELECT qty_milli FROM inventory_balances`); got != 8000 {
		t.Fatalf("stok harus 8000 milli, dapat %d", got)
	}
	if e.count(`SELECT count(*) FROM audit_logs WHERE action='sale.completed'`) != 1 {
		t.Fatal("audit sale.completed harus ada")
	}
}

func TestInsufficientStockRollsBackEverything(t *testing.T) {
	e := setup(t, 1)
	_, err := e.sell(2, 5000000)
	var ie *InsufficientError
	if !errors.As(err, &ie) || len(ie.Items) != 1 || ie.Items[0].Available != 1000 {
		t.Fatalf("harus InsufficientError dengan detail, dapat %v", err)
	}
	if e.count(`SELECT count(*) FROM sales`) != 0 || e.count(`SELECT qty_milli FROM inventory_balances`) != 1000 {
		t.Fatal("tidak boleh ada transaksi/pergerakan stok setelah gagal")
	}
}

func TestShiftNotOpen(t *testing.T) {
	e := setup(t, 5)
	_, _ = e.pool.Underlying().Exec(context.Background(), `UPDATE shifts SET status='closed', closed_at=now()`)
	if _, err := e.sell(1, 1500000); !errors.Is(err, ErrShiftNotOpen) {
		t.Fatalf("harus ErrShiftNotOpen, dapat %v", err)
	}
}

func TestPaymentTooLowIsRejected(t *testing.T) {
	e := setup(t, 5)
	var pe *PaymentError
	if _, err := e.sell(1, 1000000); !errors.As(err, &pe) {
		t.Fatalf("harus PaymentError, dapat %v", err)
	}
	if e.count(`SELECT count(*) FROM sales`) != 0 {
		t.Fatal("tidak boleh ada transaksi tersimpan")
	}
}

func TestStockMovementsAreAppendOnly(t *testing.T) {
	e := setup(t, 3)
	_, err := e.pool.Underlying().Exec(context.Background(), `DELETE FROM stock_movements`)
	if err == nil {
		t.Fatal("DELETE stock_movements harus ditolak trigger")
	}
}

// Uji konkurensi (CORE-046, §16.10): 5 kasir berebut 3 unit terakhir → tepat 3 sukses, stok tidak negatif.
func TestConcurrentSellLastStock(t *testing.T) {
	e := setup(t, 3)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, short := 0, 0
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.sell(1, 1500000)
			mu.Lock()
			defer mu.Unlock()
			var ie *InsufficientError
			switch {
			case err == nil:
				ok++
			case errors.As(err, &ie):
				short++
			default:
				t.Errorf("error tak terduga: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 3 || short != 2 {
		t.Fatalf("harus 3 sukses & 2 STOCK_INSUFFICIENT, dapat %d & %d", ok, short)
	}
	if e.count(`SELECT qty_milli FROM inventory_balances`) != 0 {
		t.Fatal("stok akhir harus 0, tidak boleh negatif")
	}
}

// 100 finalisasi paralel → nomor struk unik dan berurutan tanpa celah.
func TestReceiptNumbersGaplessUnderConcurrency(t *testing.T) {
	e := setup(t, 200)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.sell(1, 1500000); err != nil {
				t.Errorf("finalisasi gagal: %v", err)
			}
		}()
	}
	wg.Wait()
	if n := e.count(`SELECT count(DISTINCT receipt_number) FROM sales`); n != 100 {
		t.Fatalf("harus 100 nomor unik, dapat %d", n)
	}
	if mx := e.count(`SELECT last_number FROM document_counters`); mx != 100 {
		t.Fatalf("counter harus 100 tanpa celah, dapat %d", mx)
	}
}
