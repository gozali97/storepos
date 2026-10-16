package sales

import (
	"context"
	"errors"
	"testing"

	"github.com/storepos/storepos/internal/pharmacy"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

// addBatchOn membuat batch dengan ED tanggal eksplisit (YYYY-MM-DD) dan saldo SELLABLE.
func (e *env) addBatchOn(t *testing.T, number, expiry string, units int64) idkit.ID {
	t.Helper()
	ctx := context.Background()
	var id idkit.ID
	if err := e.pool.Underlying().QueryRow(ctx, `INSERT INTO pharmacy_batches(id,business_id,variant_id,batch_number,expiry_date) VALUES($1,$2,$3,$4,$5::date) RETURNING id`,
		idkit.New(), e.biz, e.variant, number, expiry).Scan(&id); err != nil {
		t.Fatal(err)
	}
	err := e.pool.InTx(ctx, func(ctx context.Context, tx database.Tx) error {
		return pharmacy.Move(ctx, tx, e.biz, e.branch, id, e.user, idkit.New(), "SELLABLE", "RECEIPT", "", units*1000)
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *env) localDate(t *testing.T, zone string, plusDays int) string {
	t.Helper()
	var d string
	if err := e.pool.Underlying().QueryRow(context.Background(), `SELECT ((clock_timestamp() AT TIME ZONE $1)::date + $2::int)::text`, zone, plusDays).Scan(&d); err != nil {
		t.Fatal(err)
	}
	return d
}

func (e *env) setZone(t *testing.T, zone string) {
	t.Helper()
	if _, err := e.pool.Underlying().Exec(context.Background(), `UPDATE branches SET timezone=$1 WHERE id=$2`, zone, e.branch); err != nil {
		t.Fatal(err)
	}
}

// APT-006: ED = hari ini (tanggal lokal cabang) tidak boleh dijual; ED besok boleh.
func TestPharmacyExpiryBoundaryIsFirstDayNotSellable(t *testing.T) {
	e := pharmacyEnv(t)
	const zone = "Asia/Jakarta"
	e.setZone(t, zone)
	e.addBatchOn(t, "TODAY", e.localDate(t, zone, 0), 5)
	var ie *InsufficientError
	if _, err := e.sellPharmacy(1, nil); !errors.As(err, &ie) {
		t.Fatalf("batch ED = hari ini tidak boleh terjual: %v", err)
	}
	e.addBatchOn(t, "TOMORROW", e.localDate(t, zone, 1), 5)
	if _, err := e.sellPharmacy(1, nil); err != nil {
		t.Fatalf("batch ED besok harus terjual: %v", err)
	}
}

// APT-006: penilaian memakai tanggal LOKAL cabang, bukan tanggal UTC server. Batch yang sama sah dijual
// di cabang yang tanggalnya masih tertinggal, dan kedaluwarsa di cabang yang tanggalnya sudah sampai.
func TestPharmacyExpiryUsesBranchLocalDateNotServerDate(t *testing.T) {
	e := pharmacyEnv(t)
	const east, west = "Pacific/Kiritimati", "Pacific/Pago_Pago" // UTC+14 dan UTC-11: tanggal lokal selalu berbeda
	b := e.addBatchOn(t, "ZONE", e.localDate(t, east, 0), 5)

	e.setZone(t, west)
	if _, err := e.sellPharmacy(1, nil); err != nil {
		t.Fatalf("di zona barat tanggalnya belum sampai ED, harus terjual: %v", err)
	}
	e.setZone(t, east)
	var ie *InsufficientError
	if _, err := e.sellPharmacy(1, nil); !errors.As(err, &ie) {
		t.Fatalf("di zona timur ED = hari ini, tidak boleh terjual: %v", err)
	}
	if got := e.batchQty(t, b, "SELLABLE"); got != 4000 {
		t.Fatalf("hanya satu penjualan yang boleh mengurangi batch: %d", got)
	}
}
