package pricing

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/storepos/storepos/internal/platform/money"
)

type vectorFile []vectorCase

type vectorCase struct {
	Name     string    `json:"name"`
	Input    vInput    `json:"input"`
	Expected vExpected `json:"expected"`
}

type vInput struct {
	Lines                []vLine   `json:"lines"`
	TransactionDiscount  vDiscount `json:"transaction_discount"`
	CashRoundingMultiple int64     `json:"cash_rounding_multiple"`
	ApplyCashRounding    bool      `json:"apply_cash_rounding"`
	RoundingMode         string    `json:"rounding_mode"`
}

type vLine struct {
	ID               string      `json:"id"`
	UnitPriceCents   string      `json:"unit_price_cents"`
	QtyMilli         string      `json:"qty_milli"`
	Modifiers        []vModifier `json:"modifiers"`
	LineDiscount     vDiscount   `json:"line_discount"`
	Tax              *vTax       `json:"tax"`
	ServiceCharge    *vSC        `json:"service_charge"`
	DiscountEligible bool        `json:"discount_eligible"`
}

type vModifier struct {
	Name            string `json:"name"`
	PriceDeltaCents string `json:"price_delta_cents"`
}

type vDiscount struct {
	Kind  string `json:"kind"`
	Value int64  `json:"value"`
}

type vTax struct {
	Name    string `json:"name"`
	RateBps int64  `json:"rate_bps"`
	Mode    string `json:"mode"`
}

type vSC struct {
	RateBps          int64 `json:"rate_bps"`
	IncludeInTaxBase bool  `json:"include_in_tax_base"`
}

type vExpected struct {
	SubtotalGrossCents       string `json:"subtotal_gross_cents"`
	TotalLineDiscountCents   string `json:"total_line_discount_cents"`
	TransactionDiscountCents string `json:"transaction_discount_cents"`
	SubtotalNetCents         string `json:"subtotal_net_cents"`
	ServiceChargeCents       string `json:"service_charge_cents"`
	TaxCents                 string `json:"tax_cents"`
	TotalBeforeRoundingCents string `json:"total_before_rounding_cents"`
	CashRoundingCents        string `json:"cash_rounding_cents"`
	TotalCents               string `json:"total_cents"`
}

func parseCentsT(t *testing.T, s string) money.Cents {
	t.Helper()
	c, err := money.ParseCents(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return c
}

func TestSharedVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatalf("baca vectors: %v", err)
	}
	var vf vectorFile
	if err := json.Unmarshal(data, &vf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, c := range vf {
		t.Run(c.Name, func(t *testing.T) {
			in := buildInput(t, c.Input)
			got, err := Compute(in)
			if err != nil {
				t.Fatalf("Compute: %v", err)
			}
			check(t, "subtotal_gross", got.SubtotalGrossCents, parseCentsT(t, c.Expected.SubtotalGrossCents))
			check(t, "line_discount", got.TotalLineDiscountCents, parseCentsT(t, c.Expected.TotalLineDiscountCents))
			check(t, "tx_discount", got.TransactionDiscountCents, parseCentsT(t, c.Expected.TransactionDiscountCents))
			check(t, "subtotal_net", got.SubtotalNetCents, parseCentsT(t, c.Expected.SubtotalNetCents))
			check(t, "service_charge", got.ServiceChargeCents, parseCentsT(t, c.Expected.ServiceChargeCents))
			check(t, "tax", got.TaxCents, parseCentsT(t, c.Expected.TaxCents))
			check(t, "total_before_rounding", got.TotalBeforeRoundingCents, parseCentsT(t, c.Expected.TotalBeforeRoundingCents))
			check(t, "cash_rounding", got.CashRoundingCents, parseCentsT(t, c.Expected.CashRoundingCents))
			check(t, "total", got.TotalCents, parseCentsT(t, c.Expected.TotalCents))
		})
	}
}

func check(t *testing.T, name string, got, want money.Cents) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %s (%d), want %s (%d)", name, got, got, want, want)
	}
}

func buildInput(t *testing.T, v vInput) Input {
	t.Helper()
	in := Input{
		TransactionDiscount:  parseDiscount(v.TransactionDiscount),
		CashRoundingMultiple: v.CashRoundingMultiple,
		ApplyCashRounding:    v.ApplyCashRounding,
		RoundingMode:         money.HalfUp,
	}
	for _, l := range v.Lines {
		line := Line{
			ID:               l.ID,
			UnitPriceCents:   parseCentsT(t, l.UnitPriceCents),
			DiscountEligible: l.DiscountEligible,
			LineDiscount:     parseDiscount(l.LineDiscount),
		}
		q, err := strconv.ParseInt(l.QtyMilli, 10, 64)
		if err != nil {
			t.Fatalf("qty: %v", err)
		}
		line.Qty = money.Milli(q)
		for _, m := range l.Modifiers {
			line.Modifiers = append(line.Modifiers, Modifier{
				Name:       m.Name,
				PriceDelta: parseCentsT(t, m.PriceDeltaCents),
			})
		}
		if l.Tax != nil {
			mode := TaxExclusive
			if l.Tax.Mode == "inclusive" {
				mode = TaxInclusive
			}
			line.Tax = &TaxRule{Name: l.Tax.Name, RateBps: l.Tax.RateBps, Mode: mode}
		}
		if l.ServiceCharge != nil {
			line.ServiceCharge = &ServiceChargeRule{
				RateBps:          l.ServiceCharge.RateBps,
				IncludeInTaxBase: l.ServiceCharge.IncludeInTaxBase,
			}
		}
		in.Lines = append(in.Lines, line)
	}
	return in
}

func parseDiscount(v vDiscount) Discount {
	switch v.Kind {
	case "amount":
		return Discount{Kind: DiscountAmount, Value: v.Value}
	case "percent":
		return Discount{Kind: DiscountPercent, Value: v.Value}
	default:
		return Discount{}
	}
}
