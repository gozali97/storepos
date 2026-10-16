package reporting

import "testing"

func TestCategoryAggregationPreservesUnknownAndVisibility(t *testing.T) {
	str := func(v string) *string { return &v }
	category := "C"
	p := []ProductRow{{ProductID: "A", CategoryID: &category, Net: str("1000"), Cost: str("300"), RestockCost: str("0"), Profit: str("700")}, {ProductID: "B", CategoryID: &category, Net: str("2000"), Cost: str("500"), RestockCost: str("100"), Profit: str("1600")}}
	rows, err := Categories(p)
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	if rows[0].ProductCount != 2 || *rows[0].Net != "3000" || *rows[0].Profit != "2300" {
		t.Fatal(rows)
	}
	p[1].MissingCost = true
	p[1].Cost = nil
	p[1].Profit = nil
	rows, err = Categories(p)
	if err != nil || rows[0].Profit != nil || rows[0].Cost != nil || rows[0].ProductsWithoutCost != 1 {
		t.Fatal(rows, err)
	}
	p[0].Net = nil
	rows, err = Categories(p)
	if err != nil || rows[0].Net != nil {
		t.Fatal(rows, err)
	}
}
