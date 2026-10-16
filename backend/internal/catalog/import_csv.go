package catalog

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

var CSVHeader = []string{"product_name", "variant_name", "sku", "base_unit", "price_cents", "barcode", "allow_fractional", "track_stock", "opening_qty_milli", "opening_cost_cents"}

type ImportRow struct {
	Number                               int
	Product, Variant, SKU, Unit, Barcode string
	Price, OpeningQty                    int64
	OpeningCost                          *int64
	Fractional, Track                    bool
}
type ImportIssue struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

func ParseImportCSV(reader io.Reader) ([]ImportRow, []ImportIssue, error) {
	r := csv.NewReader(reader)
	r.FieldsPerRecord = len(CSVHeader)
	header, err := r.Read()
	if err != nil {
		return nil, nil, err
	}
	header[0] = strings.TrimPrefix(header[0], "\ufeff")
	for i, h := range CSVHeader {
		if header[i] != h {
			return nil, nil, fmt.Errorf("header CSV harus %s", strings.Join(CSVHeader, ","))
		}
	}
	rows := []ImportRow{}
	issues := []ImportIssue{}
	skus, barcodes := map[string]bool{}, map[string]bool{}
	for number := 2; ; number++ {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if number > 5001 {
			return nil, nil, fmt.Errorf("maksimal 5000 baris data")
		}
		if err != nil {
			return nil, nil, fmt.Errorf("baris %d: %w", number, err)
		}
		for _, s := range record {
			if !utf8.ValidString(s) {
				return nil, nil, fmt.Errorf("baris %d bukan UTF-8 valid", number)
			}
		}
		x := ImportRow{Number: number, Product: strings.TrimSpace(record[0]), Variant: strings.TrimSpace(record[1]), SKU: strings.TrimSpace(record[2]), Unit: strings.TrimSpace(record[3]), Barcode: strings.TrimSpace(record[5])}
		add := func(field, msg string) { issues = append(issues, ImportIssue{number, field, msg}) }
		if x.Product == "" || utf8.RuneCountInString(x.Product) > 200 {
			add("product_name", "nama wajib maksimal 200 karakter")
		}
		if x.SKU == "" || len(x.SKU) > 100 {
			add("sku", "SKU wajib maksimal 100 byte")
		}
		if x.Unit == "" || len(x.Unit) > 100 {
			add("base_unit", "satuan wajib maksimal 100 byte")
		}
		if len(x.Barcode) > 200 {
			add("barcode", "barcode maksimal 200 byte")
		}
		if x.Price, err = strconv.ParseInt(record[4], 10, 64); err != nil || x.Price < 0 {
			add("price_cents", "harga harus integer sen tidak negatif")
		}
		if x.Price > 10000000000 {
			add("price_cents", "harga maksimal Rp100 juta")
		}
		if utf8.RuneCountInString(x.Variant) > 200 {
			add("variant_name", "nama varian maksimal 200 karakter")
		}
		if x.Fractional, err = strconv.ParseBool(record[6]); err != nil {
			add("allow_fractional", "boolean tidak valid")
		}
		if x.Track, err = strconv.ParseBool(record[7]); err != nil {
			add("track_stock", "boolean tidak valid")
		}
		if x.OpeningQty, err = strconv.ParseInt(record[8], 10, 64); err != nil || x.OpeningQty < 0 {
			add("opening_qty_milli", "stok awal harus integer milli tidak negatif")
		}
		if x.OpeningQty > 1000000000000 {
			add("opening_qty_milli", "stok awal melampaui batas impor")
		}
		if !x.Track && x.OpeningQty != 0 {
			add("opening_qty_milli", "produk tanpa pelacakan stok tidak boleh memiliki stok awal")
		}
		if !x.Fractional && x.OpeningQty%1000 != 0 {
			add("opening_qty_milli", "varian tidak mengizinkan pecahan")
		}
		if record[9] != "" {
			cost, e := strconv.ParseInt(record[9], 10, 64)
			if e != nil || cost < 0 || cost > 10000000000 {
				add("opening_cost_cents", "biaya harus integer sen tidak negatif")
			} else {
				x.OpeningCost = &cost
			}
		}
		if x.OpeningQty > 0 && x.OpeningCost == nil {
			add("opening_cost_cents", "HPP wajib untuk stok awal positif")
		}
		key := strings.ToLower(x.SKU)
		if skus[key] {
			add("sku", "SKU duplikat dalam CSV")
		}
		skus[key] = true
		if x.Barcode != "" {
			if barcodes[x.Barcode] {
				add("barcode", "barcode duplikat dalam CSV")
			}
			barcodes[x.Barcode] = true
		}
		rows = append(rows, x)
	}
	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("CSV tidak berisi data")
	}
	return rows, issues, nil
}
func ValidateImportConflicts(ctx context.Context, db database.Getter, biz idkit.ID, rows []ImportRow) ([]ImportIssue, error) {
	issues := []ImportIssue{}
	for _, x := range rows {
		var sku, barcode bool
		err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM variants WHERE business_id=$1 AND lower(sku)=lower($2)),EXISTS(SELECT 1 FROM variant_barcodes WHERE business_id=$1 AND barcode=$3)`, biz, x.SKU, x.Barcode).Scan(&sku, &barcode)
		if err != nil {
			return nil, err
		}
		if sku {
			issues = append(issues, ImportIssue{x.Number, "sku", "SKU sudah dipakai bisnis"})
		}
		if x.Barcode != "" && barcode {
			issues = append(issues, ImportIssue{x.Number, "barcode", "barcode sudah dipakai bisnis"})
		}
	}
	return issues, nil
}
