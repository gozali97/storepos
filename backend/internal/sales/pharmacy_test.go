package sales

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/storepos/storepos/internal/pharmacy"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

// pharmacyEnv: bisnis apotek dengan satu varian dan batch yang ED-nya dihitung relatif hari ini.
func pharmacyEnv(t *testing.T) *env {
	t.Helper()
	e := setup(t, 0)
	if _, err := e.pool.Underlying().Exec(context.Background(), `UPDATE businesses SET business_type='pharmacy',enabled_modules=ARRAY['pharmacy'] WHERE id=$1`, e.biz); err != nil {
		t.Fatal(err)
	}
	return e
}

// addBatch membuat batch dengan ED hari ini + days dan saldo berstatus tertentu.
func (e *env) addBatch(t *testing.T, number string, days int, status string, units int64) idkit.ID {
	t.Helper()
	ctx := context.Background()
	var id idkit.ID
	if err := e.pool.Underlying().QueryRow(ctx, `INSERT INTO pharmacy_batches(id,business_id,variant_id,batch_number,expiry_date) VALUES($1,$2,$3,$4,current_date+$5::int) RETURNING id`,
		idkit.New(), e.biz, e.variant, number, days).Scan(&id); err != nil {
		t.Fatal(err)
	}
	err := e.pool.InTx(ctx, func(ctx context.Context, tx database.Tx) error {
		return pharmacy.Move(ctx, tx, e.biz, e.branch, id, e.user, idkit.New(), status, "RECEIPT", "", units*1000)
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *env) sellPharmacy(units int64, mutate func(*Input)) (*Result, error) {
	var res *Result
	err := e.pool.InTx(context.Background(), func(ctx context.Context, tx database.Tx) error {
		in := Input{BusinessID: e.biz, BranchID: e.branch, RegisterID: e.register, UserID: e.user,
			Lines:    []LineIn{{VariantID: e.variant, QtyMilli: units * 1000}},
			Payments: []PaymentIn{{Method: "cash", AmountCents: units * 1500000}}}
		if mutate != nil {
			mutate(&in)
		}
		r, err := Finalize(ctx, tx, in)
		res = r
		return err
	})
	return res, err
}

func (e *env) batchQty(t *testing.T, batch idkit.ID, status string) int64 {
	t.Helper()
	var q int64
	err := e.pool.Underlying().QueryRow(context.Background(), `SELECT COALESCE((SELECT qty_milli FROM pharmacy_batch_balances WHERE business_id=$1 AND batch_id=$2 AND status=$3),0)`, e.biz, batch, status).Scan(&q)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func TestPharmacyFEFOAllocatesNearestExpiryAcrossBatches(t *testing.T) {
	e := pharmacyEnv(t)
	x := e.addBatch(t, "X", 30, "SELLABLE", 4)
	y := e.addBatch(t, "Y", 180, "SELLABLE", 10)
	if _, err := e.sellPharmacy(6, nil); err != nil {
		t.Fatal(err)
	}
	if got := e.batchQty(t, x, "SELLABLE"); got != 0 {
		t.Fatalf("batch ED terdekat harus habis dulu: %d", got)
	}
	if got := e.batchQty(t, y, "SELLABLE"); got != 8000 {
		t.Fatalf("sisa batch Y: %d", got)
	}
	if n := e.count(`SELECT count(*) FROM sale_line_batches`); n != 2 {
		t.Fatalf("snapshot alokasi: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM stock_movements WHERE kind='SALE'`); n != 0 {
		t.Fatalf("apotek tidak boleh memakai saldo inti: %d", n)
	}
}

func TestPharmacyNeverSellsExpiredOrQuarantinedStock(t *testing.T) {
	e := pharmacyEnv(t)
	e.addBatch(t, "OLD", 0, "SELLABLE", 5) // ED = hari ini: tidak boleh dijual
	e.addBatch(t, "PAST", -3, "SELLABLE", 5)
	e.addBatch(t, "Q", 90, "QUARANTINE", 5)
	var ie *InsufficientError
	if _, err := e.sellPharmacy(1, nil); !errors.As(err, &ie) {
		t.Fatalf("stok kedaluwarsa/karantina harus dianggap kosong: %v", err)
	}
	if n := e.count(`SELECT count(*) FROM sales`); n != 0 {
		t.Fatalf("transaksi gagal tidak boleh tersimpan: %d", n)
	}
}

func TestPharmacyPrescriptionRequiresReferenceAndPharmacist(t *testing.T) {
	e := pharmacyEnv(t)
	e.addBatch(t, "RX", 120, "SELLABLE", 5)
	ctx := context.Background()
	var product idkit.ID
	if err := e.pool.Underlying().QueryRow(ctx, `SELECT product_id FROM variants WHERE id=$1`, e.variant).Scan(&product); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Underlying().Exec(ctx, `INSERT INTO pharmacy_drugs(business_id,product_id,prescription_required) VALUES($1,$2,true)`, e.biz, product); err != nil {
		t.Fatal(err)
	}
	if _, err := e.sellPharmacy(1, nil); !errors.Is(err, ErrPrescriptionRequired) {
		t.Fatalf("tanpa resep harus ditolak: %v", err)
	}
	if _, err := e.sellPharmacy(1, func(in *Input) { in.PrescriptionRef = "RX-1" }); !errors.Is(err, ErrPrescriptionRequired) {
		t.Fatalf("referensi tanpa apoteker harus ditolak: %v", err)
	}
	if _, err := e.sellPharmacy(1, func(in *Input) { in.PrescriptionRef = "RX-1"; in.PharmacistVerified = true; in.VerifiedBy = &e.user }); err != nil {
		t.Fatalf("resep + apoteker harus lolos: %v", err)
	}
}

func TestPharmacyBatchOverrideNeedsAuthorityAndReason(t *testing.T) {
	e := pharmacyEnv(t)
	e.addBatch(t, "NEAR", 30, "SELLABLE", 5)
	far := e.addBatch(t, "FAR", 300, "SELLABLE", 5)
	override := func(can bool, reason string) func(*Input) {
		return func(in *Input) {
			in.CanOverrideBatch = can
			in.Lines[0].BatchID, in.Lines[0].BatchReason = &far, reason
		}
	}
	if _, err := e.sellPharmacy(1, override(false, "pasien minta")); !errors.Is(err, ErrBatchOverride) {
		t.Fatalf("kasir tidak boleh override: %v", err)
	}
	if _, err := e.sellPharmacy(1, override(true, "")); !errors.Is(err, ErrBatchOverride) {
		t.Fatalf("alasan wajib: %v", err)
	}
	if _, err := e.sellPharmacy(1, override(true, "ED dekat rusak")); err != nil {
		t.Fatal(err)
	}
	if got := e.batchQty(t, far, "SELLABLE"); got != 4000 {
		t.Fatalf("override harus memakai batch pilihan: %d", got)
	}
}

func TestPharmacyConcurrentSalesNeverOversellBatch(t *testing.T) {
	e := pharmacyEnv(t)
	b := e.addBatch(t, "ONE", 60, "SELLABLE", 3)
	var wg sync.WaitGroup
	results := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := e.sellPharmacy(1, nil); results <- err }()
	}
	wg.Wait()
	close(results)
	ok := 0
	for err := range results {
		var ie *InsufficientError
		if err == nil {
			ok++
		} else if !errors.As(err, &ie) {
			t.Fatal(err)
		}
	}
	if ok != 3 || e.batchQty(t, b, "SELLABLE") != 0 {
		t.Fatalf("terjual %d, sisa %d", ok, e.batchQty(t, b, "SELLABLE"))
	}
}

func TestPharmacyVoidRestoresOriginalBatch(t *testing.T) {
	e := pharmacyEnv(t)
	x := e.addBatch(t, "X", 30, "SELLABLE", 4)
	y := e.addBatch(t, "Y", 180, "SELLABLE", 10)
	r, err := e.sellPharmacy(6, nil)
	if err != nil {
		t.Fatal(err)
	}
	boss := idkit.New()
	if _, err := e.pool.Underlying().Exec(context.Background(), `INSERT INTO users(id,email,password_hash) VALUES($1,'boss@rx.test','x')`, boss); err != nil {
		t.Fatal(err)
	}
	err = e.inTx(func(ctx context.Context, tx database.Tx) error {
		return Void(ctx, tx, e.biz, r.SaleID, e.user, boss, "salah input", func(idkit.ID) bool { return true })
	})
	if err != nil {
		t.Fatal(err)
	}
	if e.batchQty(t, x, "SELLABLE") != 4000 || e.batchQty(t, y, "SELLABLE") != 10000 {
		t.Fatalf("void harus mengembalikan ke batch asal: X=%d Y=%d", e.batchQty(t, x, "SELLABLE"), e.batchQty(t, y, "SELLABLE"))
	}
}

func TestPharmacyReturnGoesToQuarantineOfOriginalBatch(t *testing.T) {
	e := pharmacyEnv(t)
	x := e.addBatch(t, "X", 30, "SELLABLE", 4)
	r, err := e.sellPharmacy(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	var line idkit.ID
	if err := e.pool.Underlying().QueryRow(context.Background(), `SELECT id FROM sale_lines WHERE sale_id=$1`, r.SaleID).Scan(&line); err != nil {
		t.Fatal(err)
	}
	err = e.inTx(func(ctx context.Context, tx database.Tx) error {
		_, err := Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: r.SaleID, UserID: e.user, RegisterID: e.register,
			Reason: "salah obat", Lines: []ReturnLine{{LineID: line, Qty: 1000, Condition: "restock"}}, HasBranch: func(idkit.ID) bool { return true }})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := e.batchQty(t, x, "QUARANTINE"); got != 1000 {
		t.Fatalf("retur harus masuk karantina batch asal: %d", got)
	}
	if got := e.batchQty(t, x, "SELLABLE"); got != 2000 {
		t.Fatalf("retur tidak boleh langsung dapat dijual: %d", got)
	}
}
