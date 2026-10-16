import { test } from 'node:test';
import assert from 'node:assert/strict';
import { BUSINESS_TYPES, termsFor } from './terms.js';

test('retail memakai istilah default "Produk"', () => {
  assert.equal(termsFor('retail').product, 'Produk');
});

test('restoran memakai istilah "Menu"', () => {
  assert.equal(termsFor('restaurant').product, 'Menu');
  assert.equal(termsFor('restaurant').products, 'Menu');
});

test('apotek memakai istilah "Obat/Produk" dan ED', () => {
  const t = termsFor('pharmacy');
  assert.equal(t.product, 'Obat/Produk');
  assert.match(t.expiry, /ED/);
});

test('jenis usaha tak dikenal jatuh ke default tanpa melempar', () => {
  const t = termsFor('tidak-ada');
  assert.equal(t.product, 'Produk');
});

test('daftar jenis usaha sesuai PRD CORE-002', () => {
  assert.deepEqual(BUSINESS_TYPES, ['retail', 'restaurant', 'pharmacy']);
});
