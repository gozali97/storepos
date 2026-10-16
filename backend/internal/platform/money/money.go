// Package money menangani uang dan kuantitas dengan integer presisi tetap.
//
// Uang disimpan sebagai int64 dalam satuan sen (1 Rupiah = 100 sen).
// Kuantitas disimpan sebagai int64 dalam milli-unit (1 unit = 1000 milli-unit),
// memberi presisi 3 desimal sesuai PRD §13.7.
//
// Floating point DILARANG untuk uang dan kuantitas di seluruh aplikasi.
package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Cents merepresentasikan nilai uang dalam satuan sen rupiah.
type Cents int64

// Milli merepresentasikan kuantitas dalam seper-seribu unit.
type Milli int64

// Qty mengembalikan Milli dari bilangan bulat unit penuh.
func Qty(units int64) Milli { return Milli(units * 1000) }

// Rupiah mengembalikan Cents dari bilangan bulat rupiah penuh.
func Rupiah(idr int64) Cents { return Cents(idr * 100) }

// String memformat Cents sebagai "Rp 1.234.567" atau "Rp 1.234,56" bila ada pecahan sen.
func (c Cents) String() string {
	neg := c < 0
	v := int64(c)
	if neg {
		v = -v
	}
	rupiah := v / 100
	sen := v % 100

	s := formatThousands(rupiah)
	if sen != 0 {
		s = fmt.Sprintf("%s,%02d", s, sen)
	}
	if neg {
		return "-Rp " + s
	}
	return "Rp " + s
}

func formatThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	first := len(s) % 3
	if first > 0 {
		b.WriteString(s[:first])
		if len(s) > first {
			b.WriteByte('.')
		}
	}
	for i := first; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte('.')
		}
	}
	return b.String()
}

// RoundingMode menentukan arah pembulatan ketika nilai jatuh di tengah.
type RoundingMode int

const (
	HalfUp RoundingMode = iota // 0,5 → naik untuk nilai positif, ke bawah untuk negatif (default PRD §13.8)
	HalfEven
	Down
	Up
)

// RoundToRupiah membulatkan ke 100 sen (rupiah penuh) sesuai PRD §13.8.
func RoundToRupiah(c Cents, mode RoundingMode) Cents {
	return roundToMultiple(int64(c), 100, mode)
}

// RoundToMultiple membulatkan ke kelipatan sen tertentu (misalnya 10_000 sen = Rp100 untuk pembulatan tunai).
func RoundToMultiple(c Cents, multipleCents int64, mode RoundingMode) Cents {
	if multipleCents <= 0 {
		return c
	}
	return roundToMultiple(int64(c), multipleCents, mode)
}

func roundToMultiple(v, m int64, mode RoundingMode) Cents {
	if m == 1 {
		return Cents(v)
	}
	q := v / m
	r := v % m
	if r == 0 {
		return Cents(q * m)
	}
	// absolut bagian sisa untuk perbandingan half
	absR := r
	if absR < 0 {
		absR = -absR
	}
	half := m / 2
	switch mode {
	case Down:
		if r < 0 {
			q--
		}
	case Up:
		if r > 0 {
			q++
		} else {
			q--
		}
	case HalfEven:
		switch {
		case absR > half:
			if r > 0 {
				q++
			} else {
				q--
			}
		case absR == half:
			if q%2 != 0 {
				if r > 0 {
					q++
				} else {
					q--
				}
			}
		}
	default: // HalfUp
		if absR*2 >= m {
			if r > 0 {
				q++
			} else {
				q--
			}
		}
	}
	return Cents(q * m)
}

// MulQty mengembalikan hasil kali price (Cents per unit) dengan qty (Milli, 3 desimal).
// Hasil masih dalam Cents (belum dibulatkan). Caller menentukan titik pembulatan.
func MulQty(price Cents, qty Milli) Cents {
	// price * qty / 1000 — dilakukan tanpa overflow untuk nilai realistis.
	return Cents(int64(price) * int64(qty) / 1000)
}

// ParseCents membaca string integer sen (dari JSON) menjadi Cents.
func ParseCents(s string) (Cents, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty cents")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid cents %q: %w", s, err)
	}
	return Cents(n), nil
}

// FormatCents mengembalikan representasi string integer untuk serialisasi JSON.
func FormatCents(c Cents) string { return strconv.FormatInt(int64(c), 10) }

// ParseMilli membaca string integer milli-unit menjadi Milli.
func ParseMilli(s string) (Milli, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty milli")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid milli %q: %w", s, err)
	}
	return Milli(n), nil
}

// FormatMilli mengembalikan representasi string integer milli-unit.
func FormatMilli(q Milli) string { return strconv.FormatInt(int64(q), 10) }

// FormatQtyDecimal menampilkan kuantitas dengan 3 desimal, trailing nol dibuang.
func FormatQtyDecimal(q Milli) string {
	neg := q < 0
	v := int64(q)
	if neg {
		v = -v
	}
	whole := v / 1000
	frac := v % 1000
	sign := ""
	if neg {
		sign = "-"
	}
	if frac == 0 {
		return sign + strconv.FormatInt(whole, 10)
	}
	s := fmt.Sprintf("%d,%03d", whole, frac)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ",")
	return sign + s
}
