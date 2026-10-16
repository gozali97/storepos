package sales

import "testing"

func TestMergeRestockCostUnknownAndEmptyBalance(t *testing.T) {
	old, cost := int64(900000), int64(500000)
	got, err := MergeRestockCost(8000, &old, 1000, &cost)
	if err != nil || got == nil || *got != 855556 {
		t.Fatal(got, err)
	}
	got, err = MergeRestockCost(0, nil, 1000, &cost)
	if err != nil || got == nil || *got != 500000 {
		t.Fatal(got, err)
	}
	got, err = MergeRestockCost(1000, nil, 1000, &cost)
	if err != nil || got != nil {
		t.Fatal("unknown old cost became zero", got, err)
	}
	got, err = MergeRestockCost(1000, &old, 1000, nil)
	if err != nil || got != nil {
		t.Fatal("unknown return cost became zero", got, err)
	}
}
