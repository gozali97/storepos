import { test } from 'node:test';
import assert from 'node:assert/strict';
import { returnLines, refundSummary } from './returns.js';

const line = { sale_line_id: 'line-a', product_name: 'Produk', remaining_qty_milli: '1500', allow_fractional: true };
test('retur hanya mengirim baris dipilih dengan qty tepat dan kondisi barang', () => {
  assert.deepEqual(returnLines([line], { 'line-a': { selected: true, qty: '0.375', condition: 'damaged' } }), [{ sale_line_id: 'line-a', qty_milli: '375', condition: 'damaged' }]);
  assert.throws(() => returnLines([line], {}));
  assert.throws(() => returnLines([line], { 'line-a': { selected: true, qty: '1.501', condition: 'restock' } }));
  assert.throws(() => returnLines([{ ...line, allow_fractional: false }], { 'line-a': { selected: true, qty: '0.5', condition: 'restock' } }));
  assert.throws(() => returnLines([line], { 'line-a': { selected: true, qty: '1', condition: 'unknown' } }));
  assert.throws(() => returnLines([line], { 'line-a': { selected: true, qty: '0', condition: 'restock' } }));
});

test('rincian refund split menandai bagian nontunai sebagai manual', () => {
  const rows = refundSummary([
    { method: 'qris', amount_cents: '1000000', manual_refund_required: true },
    { method: 'cash', amount_cents: '500000', manual_refund_required: false },
  ]);
  assert.deepEqual(rows.map(r => [r.label, r.manual]), [['QRIS', true], ['tunai', false]]);
  assert.deepEqual(refundSummary(undefined), []);
});
