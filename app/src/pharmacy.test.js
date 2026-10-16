import { test } from 'node:test';
import assert from 'node:assert/strict';
import { unitsToMilli, expiryLabel, normalizeExpiry, receiptBody, pharmacyFinalizeFields, baseQtyMilli, pharmacyErrorMessage, batchActionBody, batchCountBody, batchReportPath } from './pharmacy.js';

test('jumlah apotek dikonversi ke milli-unit dan pecahan ditolak', () => {
  assert.equal(unitsToMilli('20'), '20000');
  assert.throws(() => unitsToMilli('1.5'));
  assert.throws(() => unitsToMilli('0'));
});

test('label ED membedakan kedaluwarsa, mendekati, dan aman', () => {
  assert.equal(expiryLabel(0).tone, 'danger');
  assert.equal(expiryLabel(60).tone, 'warn');
  assert.equal(expiryLabel(91).tone, 'ok');
});

test('format ED hanya tanggal penuh atau bulan', () => {
  assert.equal(normalizeExpiry('2027-03'), '2027-03');
  assert.equal(normalizeExpiry('2027-03-31'), '2027-03-31');
  assert.throws(() => normalizeExpiry('03/2027'));
  assert.throws(() => normalizeExpiry('2027-13'));
});

test('body penerimaan batch lengkap dan status default sellable', () => {
  const body = receiptBody('b1', { variant_id: 'v', supplier_id: 's', batch_number: ' A1 ', expiry_date: '2027-03', qty: '5', unit_cost_cents: '1500' });
  assert.equal(body.batch_number, 'A1');
  assert.equal(body.qty_milli, '5000');
  assert.equal(body.status, 'SELLABLE');
  assert.throws(() => receiptBody('b1', { variant_id: 'v', supplier_id: 's', batch_number: '', expiry_date: '2027-03', qty: '5' }));
});

test('verifikasi apoteker wajib email dan PIN bersama', () => {
  assert.deepEqual(pharmacyFinalizeFields('', '', ''), {});
  assert.equal(pharmacyFinalizeFields('RX-1', 'a@b.id', '123456').pharmacist_verification.pin, '123456');
  assert.throws(() => pharmacyFinalizeFields('RX-1', 'a@b.id', '12'));
});

test('kuantitas baris dikonversi ke satuan dasar dan pecahan ditolak', () => {
  assert.equal(baseQtyMilli('2000'), '2000');
  assert.equal(baseQtyMilli('2000', '10000'), '20000'); // 2 Strip x 10 Tablet
  assert.equal(baseQtyMilli('500'), null);
});

test('pesan error apotek hanya untuk kode apotek', () => {
  assert.match(pharmacyErrorMessage('BATCH_EXPIRED'), /kedaluwarsa/);
  assert.match(pharmacyErrorMessage('PRESCRIPTION_REFERENCE_REQUIRED'), /resep/);
  assert.equal(pharmacyErrorMessage('NETWORK'), null);
});

test('body tindakan batch memvalidasi jumlah, alasan, dan status pemusnahan', () => {
  assert.deepEqual(batchActionBody('b', 'release', 'QUARANTINE', '2', ' aman '), { branch_id: 'b', qty_milli: '2000', reason: 'aman' });
  assert.equal(batchActionBody('b', 'dispose', 'SELLABLE', '1', 'rusak').status, 'SELLABLE');
  assert.throws(() => batchActionBody('b', 'dispose', 'EXPIRED', '1', 'x'));
  assert.throws(() => batchActionBody('b', 'release', 'Q', '0', 'x'));
  assert.throws(() => batchActionBody('b', 'release', 'Q', '1', ''));
});

test('body opname menerima nol dan mengubah unit dasar ke milli-unit', () => {
  assert.deepEqual(batchCountBody('b', 'SELLABLE', '0', 'hitung fisik'), { branch_id: 'b', status: 'SELLABLE', counted_milli: '0', reason: 'hitung fisik' });
  assert.equal(batchCountBody('b', 'QUARANTINE', '12', 'cek').counted_milli, '12000');
  assert.throws(() => batchCountBody('b', 'EXPIRED', '1', 'x'));
  assert.throws(() => batchCountBody('b', 'SELLABLE', '1.5', 'x'));
});

test('path laporan batch stabil dan format CSV opsional', () => {
  assert.equal(batchReportPath('b', '2026-01-01', '2026-01-31', 'expired', 30), '/reports/stock/batches?branch_id=b&from=2026-01-01&to=2026-01-31&filter=expired&days=30');
  assert.match(batchReportPath('b', '2026-01-01', '2026-01-31', 'all', 90, 'csv'), /format=csv/);
});
