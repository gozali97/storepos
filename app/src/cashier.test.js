import { test } from 'node:test';
import assert from 'node:assert/strict';
import { addProduct, cartSubtotal, cartTotals, reviewCart, isDuplicateScan, isScannerInput, discountFromInput, discountToInput, emptyCart, finalizeBody, optionalApproval, quantityToMilli } from './cashier.js';

const hit = { product_name: 'Air Mineral', variant: { id: 'variant-1', price_cents: '450000' } };

test('kuantitas kasir dikonversi tepat ke milli-unit', () => {
  assert.equal(quantityToMilli('2'), '2000');
  assert.equal(quantityToMilli('0,375'), '375');
  assert.equal(quantityToMilli('0', true), '0');
  assert.throws(() => quantityToMilli('0'));
  assert.throws(() => quantityToMilli('1.0001'));
});

test('pencegahan scan ganda memakai batas 300 ms dan dapat dimatikan', () => {
  const previous = { value: 'barcode-a', at: 1000 };
  assert.equal(isDuplicateScan(previous, 'barcode-a', 1299), true);
  assert.equal(isDuplicateScan(previous, 'barcode-a', 1300), false);
  assert.equal(isDuplicateScan(previous, 'barcode-b', 1100), false);
  assert.equal(isDuplicateScan(previous, 'barcode-a', 1100, false), false);
  assert.equal(isDuplicateScan(previous, 'barcode-a', 999), false);
});

test('deteksi HID mensyaratkan input cepat berurutan dan Enter segera', () => {
  assert.equal(isScannerInput([100, 110, 120, 130, 140], 150), true);
  assert.equal(isScannerInput([100, 200, 300, 400], 410), false);
  assert.equal(isScannerInput([100, 110, 120, 130], 250), false);
  assert.equal(isScannerInput([100, 110, 120], 130), false);
  assert.equal(isScannerInput([], 100), false);
  assert.equal(isScannerInput([100, 90, 120, 130], 140), false);
});

test('scan berulang menambah qty dan subtotal tanpa Number', () => {
  const cart = addProduct(addProduct(emptyCart(), hit), hit);
  assert.equal(cart.lines.length, 1);
  assert.equal(cart.lines[0].qty_milli, '2000');
  assert.equal(cartSubtotal(cart, new Map([[hit.variant.id, hit]])), 900000n);
});

test('diskon transaksi tersimpan langsung mengurangi subtotal contoh Rp8.000 menjadi Rp7.000', () => {
  const tea = { variant: { id: 'tea', price_cents: '800000' } };
  const products = new Map([['tea', tea]]);
  const cart = addProduct(emptyCart(), tea);
  cart.discount = discountFromInput('amount', '1000');
  const totals = cartTotals(cart, products);
  assert.equal(totals.subtotal_gross_cents, 800000n);
  assert.equal(totals.transaction_discount_cents, 100000n);
  assert.equal(cartSubtotal(cart, products), 700000n);
  cart.discount = discountFromInput('percent', '10');
  assert.equal(cartSubtotal(cart, products), 720000n);
  cart.discount = discountFromInput('none', '');
  assert.equal(cartSubtotal(cart, products), 800000n);
});

test('diskon transaksi dihitung sesudah diskon produk dan memakai harga satuan tambahan', () => {
  const unit = { unit_id: 'dus', sale_price_cents: '5000000' };
  const cart = addProduct(emptyCart(), hit, unit);
  cart.lines[0].discount = discountFromInput('amount', '5000');
  cart.discount = discountFromInput('percent', '10');
  const totals = cartTotals(cart, new Map([[hit.variant.id, hit]]), new Map([['dus', unit]]));
  assert.equal(totals.total_line_discount_cents, 500000n);
  assert.equal(totals.transaction_discount_cents, 450000n);
  assert.equal(totals.subtotal_net_cents, 4050000n);
  cart.discount = discountFromInput('amount', '100000');
  assert.equal(cartSubtotal(cart, new Map([[hit.variant.id, hit]]), new Map([['dus', unit]])), 0n);
  assert.equal(cartSubtotal(cart, new Map([[hit.variant.id, hit]])), null);
  assert.equal(cartSubtotal(emptyCart(), new Map()), 0n);
});

test('satuan tambahan dipisahkan dari satuan dasar dan scan berulang memakai harga satuan', () => {
  const unit = { unit_id: 'unit-dus', name: 'Dus', sale_price_cents: '5000000' };
  let cart = addProduct(emptyCart(), hit);
  cart = addProduct(addProduct(cart, hit, unit), hit, unit);
  assert.equal(cart.lines.length, 2);
  assert.equal(cart.lines[0].qty_milli, '1000');
  assert.equal(cart.lines[1].qty_milli, '2000');
  assert.equal(cartSubtotal(cart, new Map([[hit.variant.id, hit]]), new Map([[unit.unit_id, unit]])), 10450000n);
  assert.equal(cartSubtotal(cart, new Map([[hit.variant.id, hit]])), null);
  assert.throws(() => addProduct(cart, hit, { unit_id: 'no-price' }));
  const body = finalizeBody({ id: 'draft-a', version: 3, cart }, 'branch-a', 'register-a', []);
  assert.equal(body.lines[1].unit_id, 'unit-dus');
  assert.equal(body.lines[1].qty_milli, '2000');
  assert.equal(body.lines[1].name, undefined);
});

test('review checkout membedakan harga berubah dan barang/satuan tidak tersedia', () => {
  const products = new Map([[hit.variant.id, hit]]);
  const cart = addProduct(emptyCart(), hit);
  assert.deepEqual(reviewCart(cart, products, new Map(), products, new Map()), { unavailable: [], changed: [] });
  const changed = new Map([[hit.variant.id, { ...hit, variant: { ...hit.variant, price_cents: '500000' } }]]);
  assert.deepEqual(reviewCart(cart, products, new Map(), changed, new Map()), { unavailable: [], changed: ['Air Mineral'] });
  assert.deepEqual(reviewCart(cart, products, new Map(), new Map(), new Map()), { unavailable: ['Air Mineral'], changed: [] });
  const unit = { unit_id: 'dus-a', sale_price_cents: '5000000' };
  const withUnit = addProduct(emptyCart(), hit, unit);
  const units = new Map([[unit.unit_id, unit]]);
  assert.deepEqual(reviewCart(withUnit, products, units, products, new Map()), { unavailable: ['Air Mineral'], changed: [] });
  const newUnits = new Map([[unit.unit_id, { ...unit, sale_price_cents: '0' }]]);
  assert.deepEqual(reviewCart(withUnit, products, units, products, newUnits), { unavailable: [], changed: ['Air Mineral'] });
});

test('input diskon dikonversi tepat ke cents dan basis points', () => {
  assert.deepEqual(discountFromInput('amount', '12500.50'), { kind: 'amount', value: '1250050' });
  assert.deepEqual(discountFromInput('percent', '12,5'), { kind: 'percent', value: '1250' });
  assert.equal(discountToInput({ kind: 'amount', value: '1250050' }), '12500.5');
  assert.equal(discountToInput({ kind: 'percent', value: '1250' }), '12.5');
  assert.throws(() => discountFromInput('amount', '0'));
  assert.throws(() => discountFromInput('percent', '100.01'));
});

test('payload finalisasi hanya membawa kontrak server', () => {
  const cart = addProduct(emptyCart(), hit);
  cart.lines[0].note = 'tidak ikut finalisasi';
  const body = finalizeBody({ id: 'draft-1', version: 2, cart }, 'branch-1', 'register-1', [{ method: 'cash', amount_cents: '500000' }]);
  assert.deepEqual(body.lines[0], { variant_id: 'variant-1', qty_milli: '1000', discount: { kind: 'none', value: '0' } });
  assert.equal(body.draft_version, 2);
  assert.equal(body.customer_id, undefined);
  const withCustomer = finalizeBody({ id: 'draft-1', version: 2, cart }, 'branch-1', 'register-1', [], 'customer-1');
  assert.equal(withCustomer.customer_id, 'customer-1');
});

test('persetujuan shift harus lengkap atau dikosongkan', () => {
  assert.equal(optionalApproval('', ''), undefined);
  assert.deepEqual(optionalApproval(' owner@example.test ', '123456'), { email: 'owner@example.test', pin: '123456' });
  assert.throws(() => optionalApproval('owner@example.test', ''));
  assert.throws(() => optionalApproval('', '123456'));
});
