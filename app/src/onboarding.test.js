import { test } from 'node:test';
import assert from 'node:assert/strict';
import { stepInfo, shouldShowChecklist, progressPercent, nextStep, m2Label, onboardingEventBody } from './onboarding.js';

test('checklist tampil hanya bila belum disembunyikan dan belum selesai semua', () => {
  assert.equal(shouldShowChecklist(null), false);
  assert.equal(shouldShowChecklist({ dismissed: true, done: 1, total: 5 }), false);
  assert.equal(shouldShowChecklist({ dismissed: false, done: 5, total: 5 }), false);
  assert.equal(shouldShowChecklist({ dismissed: false, done: 2, total: 5 }), true);
});

test('progres dibulatkan dan dibatasi 0..100', () => {
  assert.equal(progressPercent(0, 5), 0);
  assert.equal(progressPercent(2, 5), 40);
  assert.equal(progressPercent(9, 5), 100);
  assert.equal(progressPercent(1, 0), 0);
});

test('langkah berikutnya adalah yang pertama belum selesai', () => {
  const items = [{ key: 'a', done: true }, { key: 'b', done: false }, { key: 'c', done: false }];
  assert.equal(nextStep(items).key, 'b');
  assert.equal(nextStep([{ key: 'a', done: true }]), null);
  assert.equal(nextStep(), null);
});

test('setiap langkah punya label dan tujuan; kunci tak dikenal tidak merusak tampilan', () => {
  assert.equal(stepInfo('first_product').to, '/produk');
  assert.equal(stepInfo('open_shift').to, '/kasir');
  assert.deepEqual(stepInfo('baru'), { label: 'baru', to: '/' });
});

test('label M2 menyesuaikan satuan dan kosong sebelum penjualan pertama', () => {
  assert.equal(m2Label(null), '');
  assert.match(m2Label(0), /kurang dari 1 menit/);
  assert.match(m2Label(25), /25 menit/);
  assert.match(m2Label(135), /2 jam 15 menit/);
});

test('event wizard hanya menerima milestone dan status whitelisted', () => {
 assert.deepEqual(onboardingEventBody('business', 'completed'), {step:'business',status:'completed'});
 assert.deepEqual(onboardingEventBody('product', 'skipped'), {step:'product',status:'skipped'});
 assert.throws(()=>onboardingEventBody('password','completed'));
 assert.throws(()=>onboardingEventBody('business','failed'));
});
