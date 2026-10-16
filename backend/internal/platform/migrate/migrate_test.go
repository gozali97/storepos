package migrate

import (
	"strings"
	"testing"
	"testing/fstest"

	"io/fs"
)

func TestStripTxnWrapper(t *testing.T) {
	in := `-- komentar header
BEGIN;

SELECT 1;

COMMIT;
`
	out := stripTxnWrapper(in)
	if strings.Contains(out, "BEGIN;") {
		t.Fatalf("BEGIN tidak dihapus:\n%s", out)
	}
	if strings.Contains(out, "COMMIT;") {
		t.Fatalf("COMMIT tidak dihapus:\n%s", out)
	}
	if !strings.Contains(out, "SELECT 1;") {
		t.Fatalf("isi migrasi hilang:\n%s", out)
	}
}

func TestStripTxnWrapperLeavesInnerTxnAlone(t *testing.T) {
	in := `BEGIN;
DO $$
BEGIN
  RAISE NOTICE 'hai';
END $$;
COMMIT;`
	out := stripTxnWrapper(in)
	// BEGIN di dalam blok DO harus tetap ada.
	if !strings.Contains(out, "  RAISE NOTICE") {
		t.Fatalf("blok DO dirusak:\n%s", out)
	}
	if strings.Contains(out, "\nBEGIN;\nDO") {
		t.Fatalf("BEGIN terluar tidak dihapus:\n%s", out)
	}
}

func TestStripTxnWrapperWithoutWrapper(t *testing.T) {
	in := "SELECT 1;\nSELECT 2;\n"
	if got := stripTxnWrapper(in); got != in {
		t.Fatalf("tanpa wrapper tidak boleh berubah:\n%q", got)
	}
}

func TestLoadParsesMigrations(t *testing.T) {
	fsys := fstest.MapFS{
		"0001_baseline.up.sql":   {Data: []byte("BEGIN;\nSELECT 1;\nCOMMIT;")},
		"0001_baseline.down.sql": {Data: []byte("BEGIN;\nSELECT 9;\nCOMMIT;")},
		"0002_catalog.up.sql":    {Data: []byte("SELECT 2;")},
	}
	r := &Runner{fsys: fsys}

	files, err := r.load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("harus ada 2 migrasi, dapat %d", len(files))
	}
	if files[0].Version != 1 || files[1].Version != 2 {
		t.Fatalf("urutan salah: %+v", files)
	}
	if files[0].Name != "baseline" {
		t.Fatalf("nama salah: %q", files[0].Name)
	}
	if files[1].DownSQL != "" {
		t.Fatal("migrasi tanpa down harus punya DownSQL kosong")
	}
}

func TestLoadRejectsBadFilename(t *testing.T) {
	fsys := fstest.MapFS{
		"baseline.sql": {Data: []byte("SELECT 1;")},
	}
	r := &Runner{fsys: fsys}
	if _, err := r.load(); err == nil {
		t.Fatal("nama berkas tanpa prefiks versi harus ditolak")
	}
}

func TestLoadRejectsMissingUp(t *testing.T) {
	fsys := fstest.MapFS{
		"0001_baseline.down.sql": {Data: []byte("SELECT 1;")},
	}
	r := &Runner{fsys: fsys}
	if _, err := r.load(); err == nil {
		t.Fatal("migrasi tanpa berkas up harus ditolak")
	}
}

func TestLoadRejectsNameClash(t *testing.T) {
	fsys := fstest.MapFS{
		"0001_baseline.up.sql": {Data: []byte("SELECT 1;")},
		"0001_lain.down.sql":   {Data: []byte("SELECT 2;")},
	}
	r := &Runner{fsys: fsys}
	if _, err := r.load(); err == nil {
		t.Fatal("versi sama dengan nama berbeda harus ditolak")
	}
}

func TestLoadSortsNumerically(t *testing.T) {
	fsys := fstest.MapFS{
		"0010_sepuluh.up.sql": {Data: []byte("SELECT 1;")},
		"0002_dua.up.sql":     {Data: []byte("SELECT 1;")},
		"0001_satu.up.sql":    {Data: []byte("SELECT 1;")},
	}
	r := &Runner{fsys: fsys}
	files, err := r.load()
	if err != nil {
		t.Fatal(err)
	}
	want := []int{1, 2, 10}
	for i, m := range files {
		if m.Version != want[i] {
			t.Fatalf("urutan numerik salah di indeks %d: %d != %d", i, m.Version, want[i])
		}
	}
}

// TestBundledMigrationsValid memastikan migrasi yang benar-benar dikirim valid
// dan dapat dibaca, sehingga kesalahan penamaan ketahuan saat `go test`.
func TestBundledMigrationsValid(t *testing.T) {
	r := &Runner{fsys: mustSub(t)}
	files, err := r.load()
	if err != nil {
		t.Fatalf("migrasi bawaan tidak valid: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("tidak ada migrasi bawaan")
	}
	for _, m := range files {
		if strings.TrimSpace(m.UpSQL) == "" {
			t.Fatalf("migrasi %04d punya up kosong", m.Version)
		}
		// Versi harus berurutan tanpa celah agar rollback prediktabel.
	}
	for i, m := range files {
		if m.Version != i+1 {
			t.Fatalf("versi migrasi harus berurutan dari 1; dapat %d di posisi %d", m.Version, i)
		}
	}
}

// mustSub membuka direktori migrasi bawaan untuk pengujian.
func mustSub(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(bundled, "sql")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	return sub
}
