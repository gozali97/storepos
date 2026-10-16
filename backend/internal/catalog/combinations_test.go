package catalog

import "testing"

func TestGenerateCombinationsAndLimits(t *testing.T) {
	out, err := Generate("KAOS", []Attribute{{Name: "Ukuran", Values: []string{"S", "M", "L"}}, {Name: "Warna", Values: []string{"Hitam", "Putih"}}})
	if err != nil || len(out) != 6 {
		t.Fatalf("generate: %+v %v", out, err)
	}
	seen := map[string]bool{}
	for _, v := range out {
		if seen[v.SKU] || len(v.Attributes) != 2 {
			t.Fatal(v)
		}
		seen[v.SKU] = true
	}
	for _, attrs := range [][]Attribute{
		{{Name: "A", Values: []string{"a", "A"}}},
		{{Name: "A", Values: []string{"1"}}, {Name: "a", Values: []string{"2"}}},
		{{Name: "A", Values: []string{"1"}}, {Name: "B", Values: []string{"2"}}, {Name: "C", Values: []string{"3"}}, {Name: "D", Values: []string{"4"}}},
	} {
		if _, err := Generate("SKU", attrs); err == nil {
			t.Fatalf("invalid accepted: %+v", attrs)
		}
	}
	values := make([]string, 11)
	for i := range values {
		values[i] = string(rune('a' + i))
	}
	if _, err := Generate("SKU", []Attribute{{"A", values}, {"B", values}}); err == nil {
		t.Fatal("121 combinations accepted")
	}
	if err := ValidateCombinations([]NewVariant{{SKU: "A", Attributes: map[string]string{"Ukuran": "M"}}, {SKU: "B", Attributes: map[string]string{"ukuran": "m"}}}); err == nil {
		t.Fatal("duplicate combination accepted")
	}
}
