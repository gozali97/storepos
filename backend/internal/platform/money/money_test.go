package money

import "testing"

func TestCentsString(t *testing.T) {
	cases := []struct {
		in   Cents
		want string
	}{
		{0, "Rp 0"},
		{100, "Rp 1"},
		{Rupiah(1_234_567), "Rp 1.234.567"},
		{Rupiah(80_850), "Rp 80.850"},
		{-Rupiah(10_000), "-Rp 10.000"},
		{12345, "Rp 123,45"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Cents(%d).String() = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRoundToRupiahHalfUp(t *testing.T) {
	cases := []struct {
		in   Cents
		want Cents
	}{
		{0, 0},
		{49, 0},
		{50, 100},
		{51, 100},
		{149, 100},
		{150, 200},
		{250, 300},
		{-50, -100},
		{-49, 0},
		{-150, -200},
	}
	for _, c := range cases {
		if got := RoundToRupiah(c.in, HalfUp); got != c.want {
			t.Errorf("RoundToRupiah(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestRoundToMultiple(t *testing.T) {
	// Pembulatan tunai Rp100 = 10_000 sen
	if got := RoundToMultiple(Rupiah(80_850), 10_000, HalfUp); got != Rupiah(80_900) {
		t.Errorf("round 80.850 to Rp100 = %s, want Rp 80.900", got)
	}
	if got := RoundToMultiple(Rupiah(80_849), 10_000, HalfUp); got != Rupiah(80_800) {
		t.Errorf("round 80.849 to Rp100 = %s, want Rp 80.800", got)
	}
}

func TestMulQty(t *testing.T) {
	// Beras Rp14.000/kg × 0,375 kg = Rp5.250 (PRD §13.8 contoh RTL-003)
	price := Rupiah(14_000)
	qty := Milli(375) // 0,375 kg
	got := MulQty(price, qty)
	want := Rupiah(5_250)
	if got != want {
		t.Fatalf("beras 0,375 kg = %s, want %s", got, want)
	}

	// 1 Dus (1000 milli = 1 unit) × Rp48.000 = Rp48.000
	if got := MulQty(Rupiah(48_000), Qty(1)); got != Rupiah(48_000) {
		t.Fatalf("1 dus = %s", got)
	}
}

func TestParseFormatCents(t *testing.T) {
	c, err := ParseCents("8085000")
	if err != nil || c != Rupiah(80_850) {
		t.Fatalf("ParseCents = %d, err=%v", c, err)
	}
	if FormatCents(Rupiah(80_850)) != "8085000" {
		t.Fatalf("FormatCents mismatch")
	}
}

func TestFormatQtyDecimal(t *testing.T) {
	cases := []struct {
		in   Milli
		want string
	}{
		{Qty(1), "1"},
		{Milli(1500), "1,5"},
		{Milli(375), "0,375"},
		{Milli(1234), "1,234"},
		{-Milli(500), "-0,5"},
	}
	for _, c := range cases {
		if got := FormatQtyDecimal(c.in); got != c.want {
			t.Errorf("FormatQtyDecimal(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
