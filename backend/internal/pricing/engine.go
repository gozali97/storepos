// Package pricing adalah mesin perhitungan harga sesuai PRD §13.8.
//
// Mesin ini menjadi SATU acuan untuk backend dan frontend. Semua angka adalah
// integer (sen untuk uang, milli-unit untuk kuantitas) agar tidak ada kehilangan
// presisi. Pembulatan half-up ke rupiah penuh (100 sen) terjadi di titik yang
// ditentukan di §13.8 dan dapat dikonfigurasi tanpa mengubah skema.
//
// Urutan perhitungan (ringkas, lihat §13.8 untuk teks lengkap):
//  1. Harga baris kotor = harga satuan × qty (+ modifier × qty), dibulatkan.
//  2. Diskon baris → neto baris sebelum diskon transaksi.
//  3. Diskon transaksi dialokasikan proporsional ke baris eligible. Pembulatan
//     per alokasi, selisih dibebankan ke baris dengan nilai eligible terbesar.
//  4. Subtotal neto = Σ neto baris.
//  5. Service charge per baris eligible, dibulatkan per baris.
//  6. Pajak eksklusif/inklusif per baris, dibulatkan per baris.
//     Pajak inklusif dikeluarkan dari neto untuk pelaporan.
//  7. Total sebelum pembulatan = subtotal neto + SC + pajak eksklusif.
//  8. Pembulatan tunai opsional ke kelipatan, dicatat sebagai rounding_adjustment.
package pricing

import (
	"errors"
	"fmt"

	"github.com/storepos/storepos/internal/platform/money"
)

// DiscountKind membedakan diskon nominal (rupiah) dan persen.
type DiscountKind int

const (
	DiscountNone DiscountKind = iota
	DiscountAmount
	DiscountPercent
)

// Discount mendefinisikan diskon baris atau transaksi.
// Untuk Percent, Value dalam basis points (1% = 100 bps) agar tetap integer.
type Discount struct {
	Kind  DiscountKind
	Value int64 // bps untuk Percent, sen untuk Amount
}

func (d Discount) apply(baseCents money.Cents) money.Cents {
	switch d.Kind {
	case DiscountAmount:
		if money.Cents(d.Value) > baseCents {
			return baseCents // dicap ke base, mesin bertanggung jawab menolak jika aturan menolak
		}
		return money.Cents(d.Value)
	case DiscountPercent:
		// base * bps / 10000, half-up ke sen (bukan rupiah)
		n := int64(baseCents) * d.Value
		q := n / 10000
		r := n % 10000
		if r*2 >= 10000 {
			q++
		} else if r*2 <= -10000 {
			q--
		}
		return money.Cents(q)
	}
	return 0
}

// TaxMode menentukan apakah harga baris sudah termasuk pajak.
type TaxMode int

const (
	TaxExclusive TaxMode = iota
	TaxInclusive
)

// TaxRule mendefinisikan satu aturan pajak yang berlaku pada baris.
type TaxRule struct {
	Name    string
	RateBps int64 // 10% = 1000 bps
	Mode    TaxMode
}

// ServiceChargeRule mendefinisikan service charge (umumnya restoran §RST-009).
type ServiceChargeRule struct {
	RateBps          int64 // 5% = 500 bps
	IncludeInTaxBase bool  // bila true, SC masuk dasar pajak (default PRD §13.8 contoh)
}

// Modifier adalah tambahan harga per unit baris (RST-004).
type Modifier struct {
	Name       string
	PriceDelta money.Cents // ≥ 0
}

// Line adalah input satu baris transaksi.
type Line struct {
	ID               string      // ID klien, dikirim balik di output
	UnitPriceCents   money.Cents // harga per satuan jual
	Qty              money.Milli // kuantitas (milli-unit)
	Modifiers        []Modifier
	LineDiscount     Discount
	Tax              *TaxRule           // nil = tidak kena pajak
	ServiceCharge    *ServiceChargeRule // nil = tidak kena SC
	DiscountEligible bool               // true = terkena diskon transaksi
}

// Input adalah payload mesin harga untuk satu transaksi.
type Input struct {
	Lines                []Line
	TransactionDiscount  Discount
	CashRoundingMultiple int64 // 0 = nonaktif; selain 0 = kelipatan sen (10000 = Rp100)
	ApplyCashRounding    bool  // true bila pembayaran tunai penuh (langkah 8)
	RoundingMode         money.RoundingMode
}

// LineResult adalah hasil perhitungan satu baris.
type LineResult struct {
	ID                 string
	GrossCents         money.Cents // langkah 1
	LineDiscountCents  money.Cents // langkah 2
	NetBeforeTxCents   money.Cents // sebelum alokasi diskon transaksi
	AllocDiscountCents money.Cents // alokasi diskon transaksi
	NetCents           money.Cents // langkah 3 akhir
	ServiceChargeCents money.Cents // langkah 5
	TaxBaseCents       money.Cents // dasar pajak (langkah 6)
	TaxCents           money.Cents // langkah 6
	LineTotalCents     money.Cents // untuk tampilan: NetCents + SC + pajak eksklusif
}

// Totals adalah ringkasan total transaksi.
type Totals struct {
	Lines                    []LineResult
	SubtotalGrossCents       money.Cents
	TotalLineDiscountCents   money.Cents
	TransactionDiscountCents money.Cents // yang terdistribusi (bisa dicap)
	SubtotalNetCents         money.Cents
	ServiceChargeCents       money.Cents
	TaxCents                 money.Cents
	TotalBeforeRoundingCents money.Cents
	CashRoundingCents        money.Cents
	TotalCents               money.Cents
}

// Compute menjalankan mesin harga. Mengembalikan Totals atau error validasi.
func Compute(in Input) (Totals, error) {
	mode := in.RoundingMode
	n := len(in.Lines)
	if n == 0 {
		return Totals{}, errors.New("pricing: minimal satu baris")
	}

	res := Totals{Lines: make([]LineResult, n)}

	// Langkah 1 & 2: gross dan diskon baris.
	for i, l := range in.Lines {
		if l.Qty <= 0 {
			return Totals{}, fmt.Errorf("pricing: baris %d qty harus > 0", i)
		}
		if l.UnitPriceCents < 0 {
			return Totals{}, fmt.Errorf("pricing: baris %d harga satuan < 0", i)
		}

		// Harga per unit termasuk modifier.
		perUnit := l.UnitPriceCents
		for _, m := range l.Modifiers {
			if m.PriceDelta < 0 {
				return Totals{}, fmt.Errorf("pricing: modifier %s delta < 0", m.Name)
			}
			perUnit += m.PriceDelta
		}
		gross := money.RoundToRupiah(money.MulQty(perUnit, l.Qty), mode)

		lineDisc := money.RoundToRupiah(l.LineDiscount.apply(gross), mode)
		if lineDisc > gross {
			return Totals{}, fmt.Errorf("pricing: diskon baris %d melebihi gross", i)
		}
		net0 := gross - lineDisc

		res.Lines[i] = LineResult{
			ID:                l.ID,
			GrossCents:        gross,
			LineDiscountCents: lineDisc,
			NetBeforeTxCents:  net0,
		}
		res.SubtotalGrossCents += gross
		res.TotalLineDiscountCents += lineDisc
	}

	// Langkah 3: diskon transaksi dialokasikan proporsional ke baris eligible.
	eligibleTotal := money.Cents(0)
	for i, l := range in.Lines {
		if l.DiscountEligible {
			eligibleTotal += res.Lines[i].NetBeforeTxCents
		}
	}
	txDiscTarget := money.Cents(0)
	if eligibleTotal > 0 {
		txDiscTarget = in.TransactionDiscount.apply(eligibleTotal)
		txDiscTarget = money.RoundToRupiah(txDiscTarget, mode)
		if txDiscTarget > eligibleTotal {
			txDiscTarget = eligibleTotal
		}
	}
	res.TransactionDiscountCents = txDiscTarget

	// Alokasikan proporsional per baris, pembulatan half-up, selisih dibebankan ke baris eligible terbesar.
	if txDiscTarget > 0 {
		var allocated money.Cents
		var maxIdx = -1
		var maxBase money.Cents
		for i, l := range in.Lines {
			if !l.DiscountEligible {
				continue
			}
			base := res.Lines[i].NetBeforeTxCents
			// alloc = txDisc * base / eligibleTotal, dibulatkan ke rupiah
			num := int64(txDiscTarget) * int64(base)
			den := int64(eligibleTotal)
			alloc := money.RoundToRupiah(money.Cents(num/den), mode)
			// Tambahkan sisa sen untuk pembulatan half-up
			rem := num % den
			// Konversi "setengah atau lebih dari den" → naik
			if rem*2 >= den {
				alloc += 100
				alloc = money.RoundToRupiah(alloc, mode)
			}
			res.Lines[i].AllocDiscountCents = alloc
			allocated += alloc
			if base > maxBase {
				maxBase = base
				maxIdx = i
			}
		}
		diff := txDiscTarget - allocated
		if diff != 0 && maxIdx >= 0 {
			res.Lines[maxIdx].AllocDiscountCents += diff
		}
	}

	for i := range res.Lines {
		res.Lines[i].NetCents = res.Lines[i].NetBeforeTxCents - res.Lines[i].AllocDiscountCents
		res.SubtotalNetCents += res.Lines[i].NetCents
	}

	// Langkah 5: service charge per baris.
	for i, l := range in.Lines {
		if l.ServiceCharge == nil {
			continue
		}
		base := res.Lines[i].NetCents
		sc := percentOf(base, l.ServiceCharge.RateBps)
		sc = money.RoundToRupiah(sc, mode)
		res.Lines[i].ServiceChargeCents = sc
		res.ServiceChargeCents += sc
	}

	// Langkah 6: pajak per baris.
	for i, l := range in.Lines {
		if l.Tax == nil {
			continue
		}
		base := res.Lines[i].NetCents
		includeSC := false
		if l.ServiceCharge != nil {
			includeSC = l.ServiceCharge.IncludeInTaxBase
		}
		if includeSC {
			base += res.Lines[i].ServiceChargeCents
		}

		var tax money.Cents
		switch l.Tax.Mode {
		case TaxExclusive:
			tax = percentOf(base, l.Tax.RateBps)
			res.Lines[i].TaxBaseCents = base
		case TaxInclusive:
			// tax = base * rate / (10000 + rate)
			num := int64(base) * l.Tax.RateBps
			den := int64(10000) + l.Tax.RateBps
			tax = money.Cents(num / den)
			rem := num % den
			if rem*2 >= den {
				tax++
			}
			res.Lines[i].TaxBaseCents = base - tax
		}
		tax = money.RoundToRupiah(tax, mode)
		res.Lines[i].TaxCents = tax
		res.TaxCents += tax
	}

	// Hitung line total untuk tampilan.
	for i := range res.Lines {
		lr := &res.Lines[i]
		exclusive := money.Cents(0)
		if in.Lines[i].Tax != nil && in.Lines[i].Tax.Mode == TaxExclusive {
			exclusive = lr.TaxCents
		}
		lr.LineTotalCents = lr.NetCents + lr.ServiceChargeCents + exclusive
	}

	// Langkah 7: total sebelum pembulatan tunai.
	// Untuk inklusif, pajak sudah ada di dalam neto, jadi tidak ditambahkan lagi.
	exclusiveTax := money.Cents(0)
	for i, l := range in.Lines {
		if l.Tax != nil && l.Tax.Mode == TaxExclusive {
			exclusiveTax += res.Lines[i].TaxCents
		}
	}
	res.TotalBeforeRoundingCents = res.SubtotalNetCents + res.ServiceChargeCents + exclusiveTax

	// Langkah 8: pembulatan tunai.
	res.TotalCents = res.TotalBeforeRoundingCents
	if in.ApplyCashRounding && in.CashRoundingMultiple > 0 {
		rounded := money.RoundToMultiple(res.TotalBeforeRoundingCents, in.CashRoundingMultiple, mode)
		res.CashRoundingCents = rounded - res.TotalBeforeRoundingCents
		res.TotalCents = rounded
	}
	return res, nil
}

func percentOf(base money.Cents, bps int64) money.Cents {
	n := int64(base) * bps
	q := n / 10000
	r := n % 10000
	if r*2 >= 10000 {
		q++
	} else if r*2 <= -10000 {
		q--
	}
	return money.Cents(q)
}
