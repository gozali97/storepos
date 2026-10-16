package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/storepos/storepos/internal/platform/config"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

// testTx membuka transaksi yang SELALU di-rollback, berisi satu bisnis uji.
// Dilewati bila database tidak tersedia, sehingga `go test` tetap hijau di CI tanpa DB.
func testTx(t *testing.T) (pgx.Tx, idkit.ID) {
	t.Helper()
	ctx := context.Background()
	// go test berjalan dari folder paket, jadi arahkan ke backend/.env secara eksplisit.
	t.Setenv("STOREPOS_ENV_FILE", "../../.env")
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("config tidak tersedia: %v", err)
	}
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("database tidak tersedia: %v", err)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Underlying().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	uid, bid := idkit.New(), idkit.New()
	if _, err := tx.Exec(ctx, `INSERT INTO users (id,email,password_hash) VALUES ($1,$2,'x')`,
		uid, uid.String()+"@uji.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO businesses (id,name,business_type,enabled_modules,created_by) VALUES ($1,'Uji','retail',ARRAY['retail'],$2)`,
		bid, uid); err != nil {
		t.Fatal(err)
	}
	return tx, bid
}

func newProd(r *Repo, bid idkit.ID, name, sku, barcode string) (*Product, error) {
	p := &Product{Name: name, TrackStock: true}
	nv := NewVariant{SKU: sku, PriceCents: 1500000}
	if barcode != "" {
		nv.Barcodes = []string{barcode}
	}
	return p, r.CreateProduct(context.Background(), bid, p, []NewVariant{nv})
}

func TestSKUUniqueCaseInsensitive(t *testing.T) {
	tx, bid := testTx(t)
	r := NewRepo(tx)
	if _, err := newProd(r, bid, "Kopi", "KOPI-01", ""); err != nil {
		t.Fatal(err)
	}
	// ON CONFLICT DO NOTHING tidak membatalkan transaksi, jadi aman di satu tx.
	if _, err := newProd(r, bid, "Kopi 2", "kopi-01", ""); !errors.Is(err, ErrSKUTaken) {
		t.Fatalf("SKU beda huruf harus ditolak, dapat %v", err)
	}
}

func TestBarcodeConflictNamesOwner(t *testing.T) {
	tx, bid := testTx(t)
	r := NewRepo(tx)
	if _, err := newProd(r, bid, "Teh Botol", "TEH-01", "899001"); err != nil {
		t.Fatal(err)
	}
	_, err := newProd(r, bid, "Teh Lain", "TEH-02", "899001")
	var bt *BarcodeTakenError
	if !errors.As(err, &bt) {
		t.Fatalf("harus BarcodeTakenError, dapat %v", err)
	}
	if bt.VariantSKU != "TEH-01" || bt.ProductName != "Teh Botol" {
		t.Fatalf("pesan harus menyebut pemilik barcode: %+v", bt)
	}
}

func TestVariantByBarcode(t *testing.T) {
	tx, bid := testTx(t)
	r := NewRepo(tx)
	if _, err := newProd(r, bid, "Gula", "GULA-1", "123456"); err != nil {
		t.Fatal(err)
	}
	v, err := r.VariantByBarcode(context.Background(), bid, " 123456 ")
	if err != nil || v.SKU != "GULA-1" || v.PriceCents != 1500000 {
		t.Fatalf("lookup barcode salah: %+v %v", v, err)
	}
	if _, err := r.VariantByBarcode(context.Background(), bid, "tidak-ada"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("harus ErrNotFound, dapat %v", err)
	}
}

func TestTenantIsolation(t *testing.T) {
	tx, bid := testTx(t)
	r := NewRepo(tx)
	if _, err := newProd(r, bid, "Rahasia", "RHS-1", "777"); err != nil {
		t.Fatal(err)
	}
	// Bisnis lain tidak boleh melihat barcode bisnis ini (NFR-013).
	if _, err := r.VariantByBarcode(context.Background(), idkit.New(), "777"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("lintas tenant harus ErrNotFound, dapat %v", err)
	}
}

func TestUpdatePriceOptimisticLock(t *testing.T) {
	tx, bid := testTx(t)
	r := NewRepo(tx)
	p, err := newProd(r, bid, "Roti", "ROTI-1", "")
	if err != nil {
		t.Fatal(err)
	}
	v := p.Variants[0]
	ctx := context.Background()

	nv, err := r.UpdatePrice(ctx, bid, v.ID, v.Version, 2000000)
	if err != nil || nv != 2 {
		t.Fatalf("update pertama harus sukses ke versi 2: %d %v", nv, err)
	}
	// Penyunting kedua masih memegang versi lama → 409.
	if _, err := r.UpdatePrice(ctx, bid, v.ID, v.Version, 2500000); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("versi lama harus ErrVersionConflict, dapat %v", err)
	}
	if _, err := r.UpdatePrice(ctx, bid, idkit.New(), 1, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("varian tak ada harus ErrNotFound, dapat %v", err)
	}
}

func TestCategoryInUseCannotBeDeleted(t *testing.T) {
	tx, bid := testTx(t)
	r := NewRepo(tx)
	ctx := context.Background()

	c, err := r.CreateCategory(ctx, bid, "Minuman")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateCategory(ctx, bid, "minuman"); !errors.Is(err, ErrCategoryTaken) {
		t.Fatalf("nama kategori ganda harus ditolak, dapat %v", err)
	}
	p := &Product{Name: "Es Teh", TrackStock: true, CategoryID: &c.ID}
	if err := r.CreateProduct(ctx, bid, p, []NewVariant{{SKU: "ET-1", PriceCents: 500000}}); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteCategory(ctx, bid, c.ID); !errors.Is(err, ErrCategoryInUse) {
		t.Fatalf("kategori terpakai harus ErrCategoryInUse, dapat %v", err)
	}
	if err := r.ArchiveCategory(ctx, bid, c.ID); err != nil {
		t.Fatalf("arsip harus berhasil: %v", err)
	}
}
