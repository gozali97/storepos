// Package seed mengisi data awal untuk lingkungan pengembangan.
package seed

import (
	"context"
	"fmt"

	"github.com/storepos/storepos/internal/platform/auth"
	"github.com/storepos/storepos/internal/platform/database"
)

const Password = "password789"

type Account struct {
	Email string
	Role  string
}

type Result struct {
	Businesses int
	Accounts   []Account
}

type businessSeed struct {
	id, name, kind, module, branchID, branchCode, registerID string
	categories                                               []categorySeed
	products                                                 []productSeed
	supplierID, customerID, codePrefix                       string
}

type categorySeed struct{ id, name string }

type productSeed struct {
	id, categoryID, variantID, name, sku, unit, barcode, purchaseUnitID, unitBarcode string
	price, stock, minimum, unitFactor, unitPrice                                     int64
	attributes                                                                       string
}

var businesses = []businessSeed{
	{
		id: "01990000-0000-7000-8000-000000000101", name: "StorePOS Retail Demo", kind: "retail", module: "retail",
		branchID: "01990000-0000-7000-8000-000000000111", branchCode: "RTL-JKT", registerID: "01990000-0000-7000-8000-000000000121", codePrefix: "RTL",
		supplierID: "01990000-0000-7000-8000-000000000131", customerID: "01990000-0000-7000-8000-000000000141",
		categories: []categorySeed{{"01990000-0000-7000-8000-000000000151", "Minuman"}, {"01990000-0000-7000-8000-000000000152", "Kebutuhan Pokok"}},
		products: []productSeed{
			{"01990000-0000-7000-8000-000000000161", "01990000-0000-7000-8000-000000000151", "01990000-0000-7000-8000-000000000171", "Air Mineral 600 ml", "RTL-AIR-600", "botol", "8999901000011", "01990000-0000-7000-8000-000000000181", "8999901000110", 400000, 72000, 12000, 12000, 4500000, `{"volume":"600 ml"}`},
			{"01990000-0000-7000-8000-000000000162", "01990000-0000-7000-8000-000000000151", "01990000-0000-7000-8000-000000000172", "Kopi Susu Botol", "RTL-KOPI-250", "botol", "8999901000028", "01990000-0000-7000-8000-000000000182", "8999901000127", 1200000, 36000, 6000, 6000, 6500000, `{"volume":"250 ml"}`},
			{"01990000-0000-7000-8000-000000000163", "01990000-0000-7000-8000-000000000152", "01990000-0000-7000-8000-000000000173", "Beras Premium 5 kg", "RTL-BERAS-5K", "karung", "8999901000035", "01990000-0000-7000-8000-000000000183", "8999901000134", 7800000, 20000, 5000, 5000, 37500000, `{"berat":"5 kg"}`},
		},
	},
	{
		id: "01990000-0000-7000-8000-000000000201", name: "StorePOS Restoran Demo", kind: "restaurant", module: "restaurant",
		branchID: "01990000-0000-7000-8000-000000000211", branchCode: "RST-JKT", registerID: "01990000-0000-7000-8000-000000000221", codePrefix: "RST",
		supplierID: "01990000-0000-7000-8000-000000000231", customerID: "01990000-0000-7000-8000-000000000241",
		categories: []categorySeed{{"01990000-0000-7000-8000-000000000251", "Makanan"}, {"01990000-0000-7000-8000-000000000252", "Minuman"}},
		products: []productSeed{
			{"01990000-0000-7000-8000-000000000261", "01990000-0000-7000-8000-000000000251", "01990000-0000-7000-8000-000000000271", "Nasi Goreng Spesial", "RST-NASGOR", "porsi", "8999902000010", "01990000-0000-7000-8000-000000000281", "8999902000119", 2800000, 50000, 10000, 10000, 25000000, `{"level_pedas":"normal"}`},
			{"01990000-0000-7000-8000-000000000262", "01990000-0000-7000-8000-000000000252", "01990000-0000-7000-8000-000000000272", "Es Teh Manis", "RST-ESTEH", "gelas", "8999902000027", "01990000-0000-7000-8000-000000000282", "8999902000126", 800000, 80000, 15000, 10000, 7000000, `{"ukuran":"regular"}`},
			{"01990000-0000-7000-8000-000000000263", "01990000-0000-7000-8000-000000000251", "01990000-0000-7000-8000-000000000273", "Ayam Bakar", "RST-AYAM", "porsi", "8999902000034", "01990000-0000-7000-8000-000000000283", "8999902000133", 3200000, 40000, 8000, 10000, 29000000, `{"sambal":"terpisah"}`},
		},
	},
	{
		id: "01990000-0000-7000-8000-000000000301", name: "StorePOS Apotek Demo", kind: "pharmacy", module: "pharmacy",
		branchID: "01990000-0000-7000-8000-000000000311", branchCode: "APT-JKT", registerID: "01990000-0000-7000-8000-000000000321", codePrefix: "APT",
		supplierID: "01990000-0000-7000-8000-000000000331", customerID: "01990000-0000-7000-8000-000000000341",
		categories: []categorySeed{{"01990000-0000-7000-8000-000000000351", "Obat Bebas"}, {"01990000-0000-7000-8000-000000000352", "Alat Kesehatan"}},
		products: []productSeed{
			{"01990000-0000-7000-8000-000000000361", "01990000-0000-7000-8000-000000000351", "01990000-0000-7000-8000-000000000371", "Paracetamol 500 mg", "APT-PCT-500", "strip", "8999903000019", "01990000-0000-7000-8000-000000000381", "8999903000118", 850000, 100000, 20000, 10000, 8000000, `{"dosis":"500 mg"}`},
			{"01990000-0000-7000-8000-000000000362", "01990000-0000-7000-8000-000000000351", "01990000-0000-7000-8000-000000000372", "Vitamin C 500 mg", "APT-VITC-500", "botol", "8999903000026", "01990000-0000-7000-8000-000000000382", "8999903000125", 3500000, 30000, 5000, 6000, 19500000, `{"isi":"30 tablet"}`},
			{"01990000-0000-7000-8000-000000000363", "01990000-0000-7000-8000-000000000352", "01990000-0000-7000-8000-000000000373", "Masker Medis", "APT-MASKER", "pcs", "8999903000033", "01990000-0000-7000-8000-000000000383", "8999903000132", 250000, 200000, 50000, 50000, 11000000, `{"warna":"biru"}`},
		},
	},
}

var accounts = []struct {
	id, email, role, businessID string
}{
	{"01990000-0000-7000-8000-000000000001", "owner@storepos.test", "owner", "01990000-0000-7000-8000-000000000101"},
	{"01990000-0000-7000-8000-000000000002", "manager@storepos.test", "manager", "01990000-0000-7000-8000-000000000101"},
	{"01990000-0000-7000-8000-000000000003", "cashier@storepos.test", "cashier", "01990000-0000-7000-8000-000000000101"},
	{"01990000-0000-7000-8000-000000000004", "warehouse@storepos.test", "warehouse", "01990000-0000-7000-8000-000000000101"},
	{"01990000-0000-7000-8000-000000000005", "kitchen@storepos.test", "kitchen", "01990000-0000-7000-8000-000000000201"},
	{"01990000-0000-7000-8000-000000000006", "pharmacist@storepos.test", "pharmacist", "01990000-0000-7000-8000-000000000301"},
}

func Apply(ctx context.Context, pool *database.Pool) (Result, error) {
	passwordHash, err := auth.Hash(Password)
	if err != nil {
		return Result{}, fmt.Errorf("hash kata sandi seed: %w", err)
	}
	result := Result{Businesses: len(businesses), Accounts: make([]Account, 0, len(accounts))}
	err = pool.InTx(ctx, func(ctx context.Context, tx database.Tx) error {
		userIDs := map[string]string{}
		for _, account := range accounts {
			var id string
			err := tx.QueryRow(ctx, `INSERT INTO users(id,email,password_hash,status,email_verified_at)
				VALUES($1,$2,$3,'active',now())
				ON CONFLICT(email) DO UPDATE SET password_hash=EXCLUDED.password_hash,status='active',updated_at=now()
				RETURNING id::text`, account.id, account.email, passwordHash).Scan(&id)
			if err != nil {
				return fmt.Errorf("seed user %s: %w", account.email, err)
			}
			userIDs[account.email] = id
			result.Accounts = append(result.Accounts, Account{Email: account.email, Role: account.role})
		}

		ownerID := userIDs["owner@storepos.test"]
		for _, business := range businesses {
			if err := seedBusiness(ctx, tx, business, ownerID); err != nil {
				return err
			}
		}

		for _, account := range accounts {
			if err := seedMembership(ctx, tx, account.businessID, userIDs[account.email], account.role); err != nil {
				return fmt.Errorf("seed membership %s: %w", account.email, err)
			}
		}
		for _, business := range businesses[1:] {
			if err := seedMembership(ctx, tx, business.id, ownerID, "owner"); err != nil {
				return fmt.Errorf("seed owner %s: %w", business.name, err)
			}
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func seedBusiness(ctx context.Context, tx database.Tx, b businessSeed, ownerID string) error {
	statements := []struct {
		name string
		sql  string
		args []any
	}{
		{"bisnis", `INSERT INTO businesses(id,name,business_type,enabled_modules,settings,created_by)
			VALUES($1,$2,$3,ARRAY[$4]::text[],'{"currency":"IDR","locale":"id-ID"}'::jsonb,$5)
			ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,business_type=EXCLUDED.business_type,enabled_modules=EXCLUDED.enabled_modules,settings=EXCLUDED.settings,updated_at=now()`, []any{b.id, b.name, b.kind, b.module, ownerID}},
		{"cabang", `INSERT INTO branches(id,business_id,code,name,timezone,address,receipt_header,receipt_footer,status)
			VALUES($1,$2,$3,'Cabang Utama','Asia/Jakarta','Jl. Contoh No. 1, Jakarta',$4,'Terima kasih telah berbelanja','active')
			ON CONFLICT(id) DO UPDATE SET code=EXCLUDED.code,name=EXCLUDED.name,timezone=EXCLUDED.timezone,address=EXCLUDED.address,receipt_header=EXCLUDED.receipt_header,receipt_footer=EXCLUDED.receipt_footer,status='active',updated_at=now()`, []any{b.branchID, b.id, b.branchCode, b.name}},
		{"terminal", `INSERT INTO registers(id,business_id,branch_id,name,status) VALUES($1,$2,$3,'Kasir 1','active')
			ON CONFLICT(id) DO UPDATE SET branch_id=EXCLUDED.branch_id,name=EXCLUDED.name,status='active',updated_at=now()`, []any{b.registerID, b.id, b.branchID}},
		{"pajak lama", `UPDATE tax_rules SET active=false WHERE business_id=$1 AND active`, []any{b.id}},
		{"pajak", `INSERT INTO tax_rules(id,business_id,name,rate_bps,mode,active) VALUES($1,$2,'PPN 11%',1100,'inclusive',true)
			ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,rate_bps=EXCLUDED.rate_bps,mode=EXCLUDED.mode,active=true`, []any{b.branchID[:len(b.branchID)-2] + "90", b.id}},
		{"supplier", `INSERT INTO suppliers(id,business_id,name,contact,address,note,archived,created_by)
			VALUES($1,$2,$3,'081234567890','Jakarta','Supplier data demo',false,$4)
			ON CONFLICT(id) DO NOTHING`, []any{b.supplierID, b.id, "Supplier " + b.codePrefix, ownerID}},
		{"pelanggan", `INSERT INTO customers(id,business_id,name,phone,note,anonymized,created_by)
			VALUES($1,$2,$3,'081200000001','Pelanggan data demo',false,$4)
			ON CONFLICT(id) DO NOTHING`, []any{b.customerID, b.id, "Pelanggan " + b.codePrefix, ownerID}},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.sql, statement.args...); err != nil {
			return fmt.Errorf("seed %s %s: %w", statement.name, b.name, err)
		}
	}

	for _, category := range b.categories {
		if _, err := tx.Exec(ctx, `INSERT INTO categories(id,business_id,name,status) VALUES($1,$2,$3,'active')
			ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,status='active',updated_at=now()`, category.id, b.id, category.name); err != nil {
			return fmt.Errorf("seed kategori %s: %w", category.name, err)
		}
	}
	for _, product := range b.products {
		if err := seedProduct(ctx, tx, b, product, ownerID); err != nil {
			return err
		}
	}
	if b.kind == "restaurant" {
		for _, category := range b.categories {
			station := category.id[:len(category.id)-2] + "91"
			name := "Dapur makanan"
			if category.name == "Minuman" {
				station = category.id[:len(category.id)-2] + "92"
				name = "Bar minuman"
			}
			if _, err := tx.Exec(ctx, `INSERT INTO kitchen_stations(id,business_id,branch_id,name) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, station, b.id, b.branchID, name); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO kitchen_category_routes(business_id,branch_id,category_id,station_id) SELECT $1,$2,$3,id FROM kitchen_stations WHERE business_id=$1 AND branch_id=$2 AND name=$4 AND NOT archived ON CONFLICT DO NOTHING`, b.id, b.branchID, category.id, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func seedProduct(ctx context.Context, tx database.Tx, b businessSeed, p productSeed, ownerID string) error {
	queries := []struct {
		name string
		sql  string
		args []any
	}{
		{"produk", `INSERT INTO products(id,business_id,category_id,name,track_stock,status) VALUES($1,$2,$3,$4,true,'active')
			ON CONFLICT(id) DO UPDATE SET category_id=EXCLUDED.category_id,name=EXCLUDED.name,track_stock=true,status='active',updated_at=now()`, []any{p.id, b.id, p.categoryID, p.name}},
		{"varian", `INSERT INTO variants(id,business_id,product_id,name,sku,base_unit,price_cents,price_includes_tax,status,allow_fractional,attributes)
			VALUES($1,$2,$3,'',$4,$5,$6,true,'active',false,$7::jsonb)
			ON CONFLICT(id) DO UPDATE SET product_id=EXCLUDED.product_id,sku=EXCLUDED.sku,base_unit=EXCLUDED.base_unit,price_cents=EXCLUDED.price_cents,price_includes_tax=true,status='active',allow_fractional=false,attributes=EXCLUDED.attributes,updated_at=now()`, []any{p.variantID, b.id, p.id, p.sku, p.unit, p.price, p.attributes}},
		{"satuan pembelian", `INSERT INTO purchase_units(id,business_id,variant_id,name,factor_milli,sale_price_cents,archived)
			VALUES($1,$2,$3,'Kemasan',$4,$5,false)
			ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,sale_price_cents=EXCLUDED.sale_price_cents,archived=false`, []any{p.purchaseUnitID, b.id, p.variantID, p.unitFactor, p.unitPrice}},
		{"barcode dasar", `INSERT INTO variant_barcodes(business_id,barcode,variant_id,unit_id) VALUES($1,$2,$3,NULL)
			ON CONFLICT(business_id,barcode) DO UPDATE SET variant_id=EXCLUDED.variant_id,unit_id=NULL`, []any{b.id, p.barcode, p.variantID}},
		{"barcode satuan", `INSERT INTO variant_barcodes(business_id,barcode,variant_id,unit_id) VALUES($1,$2,$3,$4)
			ON CONFLICT(business_id,barcode) DO UPDATE SET variant_id=EXCLUDED.variant_id,unit_id=EXCLUDED.unit_id`, []any{b.id, p.unitBarcode, p.variantID, p.purchaseUnitID}},
		{"saldo stok", `INSERT INTO inventory_balances(business_id,branch_id,variant_id,qty_milli,average_cost_cents)
			VALUES($1,$2,$3,$4,$5) ON CONFLICT(business_id,branch_id,variant_id) DO NOTHING`, []any{b.id, b.branchID, p.variantID, p.stock, p.price * 65 / 100}},
		{"stok minimum", `INSERT INTO stock_minimums(business_id,branch_id,variant_id,minimum_qty_milli,updated_by)
			VALUES($1,$2,$3,$4,$5) ON CONFLICT(business_id,branch_id,variant_id) DO UPDATE SET minimum_qty_milli=EXCLUDED.minimum_qty_milli,updated_by=EXCLUDED.updated_by,updated_at=now()`, []any{b.id, b.branchID, p.variantID, p.minimum, ownerID}},
	}
	for _, query := range queries {
		if _, err := tx.Exec(ctx, query.sql, query.args...); err != nil {
			return fmt.Errorf("seed %s %s: %w", query.name, p.name, err)
		}
	}
	return nil
}

func seedMembership(ctx context.Context, tx database.Tx, businessID, userID, role string) error {
	var membershipID string
	err := tx.QueryRow(ctx, `INSERT INTO memberships(id,user_id,business_id,role,status)
		VALUES(gen_random_uuid(),$1,$2,$3,'active')
		ON CONFLICT(user_id,business_id) DO UPDATE SET role=EXCLUDED.role,status='active',updated_at=now()
		RETURNING id::text`, userID, businessID, role).Scan(&membershipID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO membership_branches(membership_id,business_id,branch_id)
		SELECT $1,b.business_id,b.id FROM branches b WHERE b.business_id=$2
		ON CONFLICT(membership_id,branch_id) DO NOTHING`, membershipID, businessID)
	return err
}
