import { test } from 'node:test';
import assert from 'node:assert/strict';
import { productVariants } from './variants.js';

test('produk multivarian mengirim SKU/harga editable dan atribut pratinjau tanpa kehilangan presisi', () => {
  const variant = { name: 'Biru / M', sku: ' CUSTOM-001 ', price: '9007199254740993.01', attributes: { Warna: 'Biru', Ukuran: 'M' } };
  const values = { base_unit: ' pcs ', fractional: 'on' };
  const result = productVariants([variant], values);
  assert.deepEqual(result[0], { name: 'Biru / M', sku: 'CUSTOM-001', price_cents: '900719925474099301', attributes: variant.attributes, base_unit: 'pcs', allow_fractional: true, price_includes_tax: false, barcodes: [] });
  assert.throws(() => productVariants([variant, { ...variant, sku: 'custom-001' }], values));
  assert.throws(() => productVariants([{ ...variant, sku: ' ' }], values));
  assert.throws(() => productVariants([{ ...variant, price: '-1' }], values));
  assert.throws(() => productVariants([], values));
  assert.throws(() => productVariants(Array(101).fill(variant), values));
});
