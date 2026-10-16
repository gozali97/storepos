// Package sales memfinalisasi penjualan secara atomik (CORE-017, PRD §13.5):
// validasi → kunci saldo (urut ID) → hitung harga → nomor struk → simpan →
// pergerakan stok → audit, semua dalam transaksi pemanggil.
package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/inventory"
	"github.com/storepos/storepos/internal/org"
	"github.com/storepos/storepos/internal/pharmacy"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/pricing"
	"github.com/storepos/storepos/internal/purchasing"
)

const maxDeliveryFeeCents = 10_000_000_000 // Rp100 juta, sama dengan batas saat membuka pesanan

var (
	ErrShiftNotOpen       = errors.New("shift belum dibuka pada terminal ini")
	ErrShiftAlreadyOpen   = errors.New("terminal ini sudah punya shift terbuka")
	ErrNoLines            = errors.New("transaksi tanpa baris")
	ErrInvalidQty         = errors.New("kuantitas harus > 0")
	ErrInvalidDeliveryFee = errors.New("ongkir tidak valid: harus >= 0 dan maksimal Rp100.000.000")
	ErrInvalidNote        = errors.New("catatan maksimal 200 karakter dan harus UTF-8 valid")
	ErrVariantUnavailable = errors.New("produk tidak ditemukan atau tidak aktif")
	ErrNotFound           = errors.New("cabang atau terminal tidak ditemukan")
	ErrZeroTotal          = errors.New("total transaksi harus > 0")
	ErrInvalidDiscount    = errors.New("diskon tidak valid")
	ErrDiscountExceeds    = errors.New("diskon melebihi nilai yang didiskon")
)

// PaymentError: pembayaran tidak valid terhadap total.
type PaymentError struct{ Msg string }

func (e *PaymentError) Error() string { return e.Msg }

type Shortage struct {
	VariantID, SKU, ProductName string
	Requested, Available        int64
}

// InsufficientError memuat semua item yang stoknya kurang (STOCK_INSUFFICIENT).
type InsufficientError struct{ Items []Shortage }

func (e *InsufficientError) Error() string { return "stok tidak mencukupi" }

// OrderSnapshot is internal-only: never accepted from retail request JSON.
type OrderSnapshot struct {
	Price                  int64
	Product, Variant, Unit string
	Modifiers              json.RawMessage
	Delta                  int64
	Track                  bool
}

type LineIn struct {
	OrderSnapshot *OrderSnapshot
	Note          string
	VariantID     idkit.ID
	UnitID        *idkit.ID
	QtyMilli      int64
	Discount      pricing.Discount // diskon baris (CORE-015)
	BatchID       *idkit.ID        // override batch manual apotek (APT-005)
	BatchReason   string
}

type PaymentIn struct {
	Method      string
	AmountCents int64 // jumlah yang diserahkan pelanggan
}

type Input struct {
	ServiceCharge                            *pricing.ServiceChargeRule
	Note                                     string
	BusinessID, BranchID, RegisterID, UserID idkit.ID
	Lines                                    []LineIn
	Payments                                 []PaymentIn

	// Diskon transaksi dan kebijakannya (CORE-015). Batas dalam bps dari subtotal kotor;
	// diskon di atas batas butuh DiscountApprovedBy (PIN atasan sudah diverifikasi pemanggil).
	TxDiscount         pricing.Discount
	DiscountReason     string
	DiscountApprovedBy *idkit.ID
	UnlimitedDiscount  bool
	DiscountLimitBps   int64

	// Apotek: referensi resep, verifikasi apoteker (APT-008/009) dan izin override batch (APT-005).
	PrescriptionRef    string
	PharmacistVerified bool
	VerifiedBy         *idkit.ID
	CanOverrideBatch   bool

	// Ongkir restoran: biaya terpisah, tidak kena pajak/diskon/service charge, ikut total dan pembulatan tunai.
	DeliveryFeeCents int64
}

type Result struct {
	SaleID                                               idkit.ID
	ReceiptNumber                                        string
	TotalCents, ChangeCents, CashRoundingCents, TaxCents int64
	DeliveryFeeCents                                     int64
}

// OpenShift membuka shift pada terminal (CORE-024).
func OpenShift(ctx context.Context, db database.Getter, biz, branch, register, user idkit.ID, openingCents int64) (idkit.ID, error) {
	if openingCents < 0 {
		return idkit.ID{}, errors.New("kas awal tidak boleh negatif")
	}
	var status string
	err := db.QueryRow(ctx, `SELECT r.status FROM registers r JOIN branches b ON b.business_id=r.business_id AND b.id=r.branch_id WHERE r.business_id=$1 AND r.branch_id=$2 AND r.id=$3 AND b.status='active' FOR SHARE OF b FOR UPDATE OF r`,
		biz, branch, register).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && status != "active") {
		return idkit.ID{}, ErrNotFound
	}
	if err != nil {
		return idkit.ID{}, fmt.Errorf("sales: baca terminal: %w", err)
	}
	var open bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM shifts WHERE register_id=$1 AND status='open')`, register).Scan(&open); err != nil {
		return idkit.ID{}, fmt.Errorf("sales: cek shift: %w", err)
	}
	if open {
		return idkit.ID{}, ErrShiftAlreadyOpen
	}
	id := idkit.New()
	if _, err := db.Exec(ctx,
		`INSERT INTO shifts (id,business_id,branch_id,register_id,opened_by,opening_cash_cents) VALUES ($1,$2,$3,$4,$5,$6)`,
		id, biz, branch, register, user, openingCents); err != nil {
		return idkit.ID{}, fmt.Errorf("sales: buka shift: %w", err)
	}
	return id, nil
}

type vinfo struct {
	SKU, ProductName, VariantName, Unit string
	Price                               int64
	Track                               bool
	Fractional                          bool
	ProductID                           idkit.ID
	CategoryID                          *idkit.ID
}

func loadVariant(ctx context.Context, db database.Getter, biz, id idkit.ID) (vinfo, error) {
	var v vinfo
	err := db.QueryRow(ctx,
		`SELECT v.sku, p.name, v.name, v.base_unit, v.price_cents, p.track_stock, v.allow_fractional, p.id, p.category_id
		 FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id
		 WHERE v.business_id=$1 AND v.id=$2 AND v.status='active' AND p.status='active'`, biz, id).
		Scan(&v.SKU, &v.ProductName, &v.VariantName, &v.Unit, &v.Price, &v.Track, &v.Fractional, &v.ProductID, &v.CategoryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrVariantUnavailable
	}
	if err != nil {
		return v, fmt.Errorf("sales: baca varian: %w", err)
	}
	return v, nil
}

// Finalize menyelesaikan penjualan. Harga selalu dibaca dari server, bukan klien.
// shortcut: service charge belum tersedia; tambah saat modul restoran RST-009 dikerjakan.
// Pembulatan tunai mengikuti kebijakan bisnis dan hanya berlaku bila semua pembayaran tunai.
func Finalize(ctx context.Context, db database.Getter, in Input) (*Result, error) {
	var isPharmacy bool
	if err := db.QueryRow(ctx, `SELECT business_type='pharmacy' FROM businesses WHERE id=$1`, in.BusinessID).Scan(&isPharmacy); err != nil {
		return nil, err
	}
	if !utf8.ValidString(in.Note) || utf8.RuneCountInString(in.Note) > 200 {
		return nil, ErrInvalidNote
	}
	for _, l := range in.Lines {
		if !utf8.ValidString(l.Note) || utf8.RuneCountInString(l.Note) > 200 {
			return nil, ErrInvalidNote
		}
	}
	if len(in.Lines) == 0 {
		return nil, ErrNoLines
	}
	// Claim the type lock in this same transaction; failed sales roll it back.
	// Tenant row comes first so settings changes and concurrent first sales serialize safely.
	if _, err := db.Exec(ctx, `UPDATE businesses SET first_sale_completed_at=COALESCE(first_sale_completed_at,clock_timestamp()) WHERE id=$1 AND first_sale_completed_at IS NULL`, in.BusinessID); err != nil {
		return nil, err
	}
	// FOR SHARE: tutup shift (FOR UPDATE) menunggu penjualan yang sedang berjalan, dan sebaliknya.
	shiftID, _, _, err := lockOpenShift(ctx, db, in.BusinessID, in.BranchID, in.RegisterID, "FOR SHARE")
	if err != nil {
		return nil, err
	}
	var code, tz string
	if err := db.QueryRow(ctx, `SELECT code, timezone FROM branches WHERE business_id=$1 AND id=$2`,
		in.BusinessID, in.BranchID).Scan(&code, &tz); err != nil {
		return nil, ErrNotFound
	}

	if !validDiscount(in.TxDiscount) {
		return nil, ErrInvalidDiscount
	}
	tax, err := loadTax(ctx, db, in.BusinessID)
	if err != nil {
		return nil, err
	}
	infos := make([]vinfo, len(in.Lines))
	plines := make([]pricing.Line, len(in.Lines))
	need := map[idkit.ID]int64{}
	tracked := map[idkit.ID]vinfo{}
	baseQty := make([]int64, len(in.Lines))
	factors := make([]int64, len(in.Lines))
	saleUnits := make([]string, len(in.Lines))
	var pl []pharmLine
	for i, l := range in.Lines {
		if l.QtyMilli <= 0 {
			return nil, ErrInvalidQty
		}
		v, err := loadVariant(ctx, db, in.BusinessID, l.VariantID)
		if err != nil {
			return nil, err
		}
		if snap := l.OrderSnapshot; snap != nil {
			v.Price = snap.Price + snap.Delta
			v.ProductName = snap.Product
			v.VariantName = snap.Variant
			v.Unit = snap.Unit
			v.Track = snap.Track
		}
		if !v.Fractional && l.QtyMilli%1000 != 0 {
			return nil, ErrInvalidQty
		}
		factors[i] = 1000
		saleUnits[i] = v.Unit
		baseQty[i] = l.QtyMilli
		if l.UnitID != nil {
			var price *int64
			err := db.QueryRow(ctx, `SELECT name,factor_milli,sale_price_cents FROM purchase_units WHERE business_id=$1 AND variant_id=$2 AND id=$3 AND NOT archived FOR UPDATE`, in.BusinessID, l.VariantID, *l.UnitID).Scan(&saleUnits[i], &factors[i], &price)
			if errors.Is(err, pgx.ErrNoRows) || err == nil && price == nil {
				return nil, ErrVariantUnavailable
			}
			if err != nil {
				return nil, err
			}
			baseQty[i], err = purchasing.Convert(l.QtyMilli, factors[i])
			if err != nil {
				return nil, ErrInvalidQty
			}
			v.Price = *price
			if _, err := db.Exec(ctx, `UPDATE purchase_units SET used_at=COALESCE(used_at,now()) WHERE business_id=$1 AND id=$2`, in.BusinessID, *l.UnitID); err != nil {
				return nil, err
			}
		}
		infos[i] = v
		if !validDiscount(l.Discount) {
			return nil, ErrInvalidDiscount
		}
		lineTax, err := EffectiveTax(ctx, db, in.BusinessID, v.ProductID, tax)
		if err != nil {
			return nil, err
		}
		plines[i] = pricing.Line{ID: strconv.Itoa(i), UnitPriceCents: money.Cents(v.Price),
			Qty: money.Milli(l.QtyMilli), LineDiscount: l.Discount, Tax: lineTax, ServiceCharge: in.ServiceCharge, DiscountEligible: true}
		if v.Track && isPharmacy {
			if l.BatchID != nil && (!in.CanOverrideBatch || strings.TrimSpace(l.BatchReason) == "") {
				return nil, ErrBatchOverride
			}
			pl = append(pl, pharmLine{idx: i, variant: l.VariantID, qty: baseQty[i], override: l.BatchID, reason: strings.TrimSpace(l.BatchReason)})
		} else if v.Track {
			need[l.VariantID] += baseQty[i]
			tracked[l.VariantID] = v
		}
	}

	ids := make([]idkit.ID, 0, len(need))
	for id := range need {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	var short []Shortage
	costs := map[idkit.ID]*int64{}
	for _, id := range ids {
		avail, err := inventory.Lock(ctx, db, in.BusinessID, in.BranchID, id)
		if err != nil {
			return nil, err
		}
		var cost *int64
		if err := db.QueryRow(ctx, `SELECT average_cost_cents FROM inventory_balances WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, in.BusinessID, in.BranchID, id).Scan(&cost); err != nil {
			return nil, err
		}
		costs[id] = cost
		var reserved int64
		if err := db.QueryRow(ctx, `SELECT reserved_milli FROM inventory_balances WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, in.BusinessID, in.BranchID, id).Scan(&reserved); err != nil {
			return nil, err
		}
		avail -= reserved
		if avail < need[id] {
			v := tracked[id]
			short = append(short, Shortage{id.String(), v.SKU, v.ProductName, need[id], avail})
		}
	}
	if len(short) > 0 {
		return nil, &InsufficientError{Items: short}
	}
	var allocs map[int][]batchAlloc
	if len(pl) > 0 {
		today, err := pharmacy.BranchDate(ctx, db, in.BusinessID, in.BranchID)
		if err != nil {
			return nil, err
		}
		if err := checkPrescription(ctx, db, in, infos, pl); err != nil {
			return nil, err
		}
		if allocs, err = allocateBatches(ctx, db, in, today.Format("2006-01-02"), infos, pl); err != nil {
			return nil, err
		}
	}

	methods, _, err := org.LoadPaymentMethods(ctx, db, in.BusinessID)
	if err != nil {
		return nil, err
	}
	enabledMethods := map[string]bool{}
	for _, method := range methods {
		enabledMethods[method.Code] = method.Enabled
	}
	var cash, nonCash int64
	for _, p := range in.Payments {
		if !enabledMethods[p.Method] {
			return nil, &PaymentError{"metode pembayaran tidak aktif atau tidak dikenal: " + p.Method}
		}
		if p.AmountCents <= 0 {
			return nil, &PaymentError{"jumlah pembayaran harus > 0"}
		}
		switch p.Method {
		case "cash":
			cash += p.AmountCents
		case "qris", "card", "transfer":
			nonCash += p.AmountCents
		default:
			return nil, &PaymentError{"metode pembayaran tidak dikenal: " + p.Method}
		}
	}
	allCash := len(in.Payments) > 0 && nonCash == 0
	policy, err := org.LoadPolicy(ctx, db, in.BusinessID)
	if err != nil {
		return nil, err
	}
	fee := in.DeliveryFeeCents
	if fee < 0 || fee > maxDeliveryFeeCents {
		return nil, ErrInvalidDeliveryFee
	}
	applyRounding := allCash && policy.CashRoundingEnabled
	totals, err := pricing.Compute(pricing.Input{Lines: plines, TransactionDiscount: in.TxDiscount,
		CashRoundingMultiple: policy.CashRoundingMultipleCents, ApplyCashRounding: applyRounding && fee == 0})
	if err != nil {
		return nil, fmt.Errorf("sales: hitung harga: %w", err)
	}
	if err := checkDiscounts(in, totals); err != nil {
		return nil, err
	}
	total := int64(totals.TotalCents)
	rounding := int64(totals.CashRoundingCents)
	if fee > 0 {
		// Ongkir ditambahkan setelah mesin harga; pembulatan tunai dihitung atas total termasuk ongkir.
		total += fee
		if applyRounding && policy.CashRoundingMultipleCents > 0 {
			rounded := int64(money.RoundToMultiple(money.Cents(total), policy.CashRoundingMultipleCents, money.HalfUp))
			rounding = rounded - total
			total = rounded
		}
	}
	if total <= 0 {
		return nil, ErrZeroTotal
	}
	if nonCash > total {
		return nil, &PaymentError{"pembayaran nontunai melebihi total"}
	}
	remaining := total - nonCash
	if cash < remaining {
		return nil, &PaymentError{"pembayaran kurang dari total"}
	}
	if cash > 0 && remaining == 0 {
		return nil, &PaymentError{"pembayaran tunai tidak diperlukan"}
	}
	change := cash - remaining // kembalian hanya dari tunai (CORE-021)

	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	var seq int
	if err := db.QueryRow(ctx,
		`INSERT INTO document_counters (business_id,branch_id,doc_type,doc_date,last_number)
		 VALUES ($1,$2,'receipt',$3,1)
		 ON CONFLICT (business_id,branch_id,doc_type,doc_date)
		 DO UPDATE SET last_number = document_counters.last_number + 1, updated_at = now()
		 RETURNING last_number`,
		in.BusinessID, in.BranchID, time.Now().In(loc).Format("2006-01-02")).Scan(&seq); err != nil {
		return nil, fmt.Errorf("sales: nomor struk: %w", err)
	}
	receipt := fmt.Sprintf("%s-%s-%04d", code, time.Now().In(loc).Format("060102"), seq)

	// Snapshot aturan pajak: perubahan tarif kelak tidak mengubah transaksi ini (CORE-016).
	var taxName, taxBps, taxMode, discReason any
	if tax != nil {
		taxName, taxBps, taxMode = tax.Name, tax.RateBps, "exclusive"
		if tax.Mode == pricing.TaxInclusive {
			taxMode = "inclusive"
		}
	}
	if r := strings.TrimSpace(in.DiscountReason); r != "" {
		discReason = r
	}
	var header []byte
	if err := db.QueryRow(ctx, `SELECT jsonb_build_object('business_name',t.name,'branch_name',b.name,'branch_code',b.code,'timezone',b.timezone,'address',b.address,'receipt_header',b.receipt_header,'receipt_footer',b.receipt_footer,'cashier_name',NULLIF(u.display_name,''))
	FROM businesses t JOIN branches b ON b.business_id=t.id JOIN users u ON u.id=$3 WHERE t.id=$1 AND b.id=$2`, in.BusinessID, in.BranchID, in.UserID).Scan(&header); err != nil {
		return nil, err
	}
	saleID := idkit.New()
	if _, err := db.Exec(ctx,
		`INSERT INTO sales (id,business_id,branch_id,register_id,shift_id,receipt_number,subtotal_gross_cents,
		   line_discount_cents,tx_discount_cents,service_charge_cents,tax_cents,cash_rounding_cents,total_cents,change_cents,created_by,
		   tax_rule_name,tax_rate_bps,tax_mode,discount_reason,discount_approved_by,receipt_header_snapshot,note,delivery_fee_cents)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`,
		saleID, in.BusinessID, in.BranchID, in.RegisterID, shiftID, receipt, int64(totals.SubtotalGrossCents),
		int64(totals.TotalLineDiscountCents), int64(totals.TransactionDiscountCents), int64(totals.ServiceChargeCents),
		int64(totals.TaxCents), rounding, total, change, in.UserID,
		taxName, taxBps, taxMode, discReason, in.DiscountApprovedBy, header, in.Note, fee); err != nil {
		return nil, fmt.Errorf("sales: simpan transaksi: %w", err)
	}
	if isPharmacy {
		if _, err := db.Exec(ctx, `UPDATE sales SET prescription_reference=NULLIF($3,''),pharmacist_verified_by=$4 WHERE business_id=$1 AND id=$2`, in.BusinessID, saleID, strings.TrimSpace(in.PrescriptionRef), in.VerifiedBy); err != nil {
			return nil, err
		}
	}
	for i, l := range in.Lines {
		r, v := totals.Lines[i], infos[i]
		var lineTaxName, lineTaxRate, lineTaxMode any
		if t := plines[i].Tax; t != nil {
			lineTaxName = t.Name
			lineTaxRate = t.RateBps
			lineTaxMode = "exclusive"
			if t.Mode == pricing.TaxInclusive {
				lineTaxMode = "inclusive"
			}
		}
		var cost *int64
		if price := costs[l.VariantID]; price != nil {
			value, err := SnapshotCost(*price, baseQty[i])
			if err != nil {
				return nil, err
			}
			cost = &value
		}
		mods := json.RawMessage(`[]`)
		if l.OrderSnapshot != nil {
			mods = l.OrderSnapshot.Modifiers
		}
		lineID := idkit.New()
		if _, err := db.Exec(ctx,
			`INSERT INTO sale_lines (id,business_id,sale_id,line_no,variant_id,sku,product_name,variant_name,base_unit,qty_milli,
			   unit_price_cents,gross_cents,line_discount_cents,alloc_discount_cents,net_cents,service_charge_cents,tax_cents,line_total_cents,allow_fractional,unit_id,sale_unit,factor_milli,base_qty_milli,cost_cents,product_id,category_id,note,tax_rule_name,tax_rate_bps,tax_mode,modifiers)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31)`,
			lineID, in.BusinessID, saleID, i+1, l.VariantID, v.SKU, v.ProductName, v.VariantName, v.Unit, l.QtyMilli,
			v.Price, int64(r.GrossCents), int64(r.LineDiscountCents), int64(r.AllocDiscountCents), int64(r.NetCents),
			int64(r.ServiceChargeCents), int64(r.TaxCents), int64(r.LineTotalCents), v.Fractional, l.UnitID, saleUnits[i], factors[i], baseQty[i], cost, v.ProductID, v.CategoryID, l.Note, lineTaxName, lineTaxRate, lineTaxMode, mods); err != nil {
			return nil, fmt.Errorf("sales: simpan baris: %w", err)
		}
		if v.Track && isPharmacy {
			if err := recordBatchSale(ctx, db, in, saleID, lineID, allocs[i]); err != nil {
				return nil, err
			}
		} else if v.Track {
			if _, err := inventory.Apply(ctx, db, inventory.Movement{BusinessID: in.BusinessID, BranchID: in.BranchID,
				VariantID: l.VariantID, Kind: inventory.KindSale, QtyMilli: -baseQty[i], RefType: "sale", RefID: saleID, UserID: in.UserID}); err != nil {
				return nil, err
			}
		}
	}
	insertPay := func(method string, amount, tendered, chg int64) error {
		_, err := db.Exec(ctx,
			`INSERT INTO payments (id,business_id,sale_id,method,amount_cents,tendered_cents,change_cents) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			idkit.New(), in.BusinessID, saleID, method, amount, tendered, chg)
		return err
	}
	for _, p := range in.Payments {
		if p.Method != "cash" {
			if err := insertPay(p.Method, p.AmountCents, p.AmountCents, 0); err != nil {
				return nil, fmt.Errorf("sales: simpan pembayaran: %w", err)
			}
		}
	}
	if cash > 0 {
		if err := insertPay("cash", remaining, cash, change); err != nil {
			return nil, fmt.Errorf("sales: simpan pembayaran: %w", err)
		}
	}
	if err := audit.Write(ctx, db, audit.Entry{BusinessID: in.BusinessID, BranchID: &in.BranchID, RegisterID: &in.RegisterID,
		ActorUserID: in.UserID, ApprovedBy: in.DiscountApprovedBy, Action: "sale.completed", EntityType: "sale", EntityID: saleID.String(),
		After: map[string]any{"receipt_number": receipt, "total_cents": total,
			"discount_cents": int64(totals.TotalLineDiscountCents + totals.TransactionDiscountCents)}, Reason: strings.TrimSpace(in.DiscountReason)}); err != nil {
		return nil, err
	}
	return &Result{SaleID: saleID, ReceiptNumber: receipt, TotalCents: total, ChangeCents: change,
		CashRoundingCents: rounding, TaxCents: int64(totals.TaxCents), DeliveryFeeCents: fee}, nil
}
