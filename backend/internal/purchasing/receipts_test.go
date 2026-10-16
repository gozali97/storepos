package purchasing

import "testing"

func TestConversionAndMovingAverage(t *testing.T) {
	if qty, err := Convert(2000, 24000); err != nil || qty != 48000 {
		t.Fatalf("2 dus: %d %v", qty, err)
	}
	if _, err := Convert(1, 1); err == nil {
		t.Fatal("precision loss accepted")
	}
	for _, c := range []struct{ stock, old, qty, price, want int64 }{{0, 0, 48000, 200000, 200000}, {48000, 200000, 48000, 400000, 300000}, {-1, 1, 1000, 900, 900}} {
		got, err := AverageCost(c.stock, c.old, c.qty, c.price)
		if err != nil || got != c.want {
			t.Fatalf("average: %d %v", got, err)
		}
	}
	if _, err := AverageCost(1, 0, 0, 1); err == nil {
		t.Fatal("zero quantity accepted")
	}
}
