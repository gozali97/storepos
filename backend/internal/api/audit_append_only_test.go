package apiserver

import (
	"strings"
	"testing"

	"github.com/storepos/storepos/internal/platform/testdb"
)

// CORE-042: audit log tidak dapat diubah, dihapus, maupun dikosongkan lewat SQL langsung.
func TestAuditLogsRejectUpdateDeleteAndTruncateAtDatabaseLevel(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@auditdb.test")
	base := newServer(t, pool)
	login(t, base, "owner@auditdb.test", biz.String()) // menghasilkan baris audit auth.login

	db := pool.Underlying()
	var before int
	if err := db.QueryRow(t.Context(), `SELECT count(*) FROM audit_logs`).Scan(&before); err != nil || before == 0 {
		t.Fatalf("butuh minimal satu baris audit: %v %d", err, before)
	}
	// Urutan tetap: setiap pernyataan harus ditolak dan baris tetap ada untuk pernyataan berikutnya.
	statements := []struct{ name, sql string }{
		{"update alasan", `UPDATE audit_logs SET reason='dihapus'`},
		{"update aktor", `UPDATE audit_logs SET actor_user_id=NULL`},
		{"delete semua", `DELETE FROM audit_logs`},
		{"truncate", `TRUNCATE audit_logs`},
	}
	for _, s := range statements {
		_, err := db.Exec(t.Context(), s.sql)
		if err == nil {
			t.Errorf("%s: seharusnya ditolak database", s.name)
			continue
		}
		if !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%s: pesan error tak terduga: %v", s.name, err)
		}
	}
	// Pergerakan stok juga append-only (CORE-032); TRUNCATE tidak boleh menghapus kartu stok.
	for _, table := range []string{"stock_movements", "pharmacy_batch_movements"} {
		if _, err := db.Exec(t.Context(), `TRUNCATE `+table); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("truncate %s harus ditolak: %v", table, err)
		}
	}
	var after int
	if err := db.QueryRow(t.Context(), `SELECT count(*) FROM audit_logs`).Scan(&after); err != nil || after != before {
		t.Fatalf("jumlah baris audit berubah: %d -> %d (%v)", before, after, err)
	}
}
