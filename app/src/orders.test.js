import { test } from 'node:test';
import assert from 'node:assert/strict';
import { addLineBody, checkoutPayments, liveLines, lineStatusLabel, modifierError, notSentCount, paymentBlockReason, cancelNeedsApproval, cancelBody, availabilityRequest, availabilityLabel, splitCheckoutPayments, editLineBody, orderDiscountBody } from './orders.js';

test('baris belum dikirim dihitung dan baris dibatalkan disaring', () => {
  const lines = [{ kitchen_status: 'NOT_SENT' }, { kitchen_status: 'SENT' }, { kitchen_status: 'CANCELLED' }, { kitchen_status: 'NOT_SENT' }];
  assert.equal(notSentCount(lines), 2);
  assert.equal(liveLines(lines).length, 3);
  assert.equal(notSentCount(), 0);
});

test('body tambah baris memakai milli-unit dan menolak catatan terlalu panjang', () => {
  assert.deepEqual(addLineBody('v1', '2', ' tanpa gula ', ['o1']), { variant_id: 'v1', qty_milli: '2000', note: 'tanpa gula', modifier_option_ids: ['o1'] });
  assert.deepEqual(addLineBody('v1', '1'), { variant_id: 'v1', qty_milli: '1000' });
  assert.throws(() => addLineBody('', '1'));
  assert.throws(() => addLineBody('v1', '1', 'x'.repeat(141)));
});

test('pilihan modifier divalidasi terhadap batas grup', () => {
  const groups = [{ name: 'Gula', min_choices: 1, max_choices: 1, options: [{ option_id: 'a' }, { option_id: 'b' }] }];
  assert.match(modifierError(groups, []), /minimal 1/);
  assert.equal(modifierError(groups, ['a']), '');
  assert.match(modifierError(groups, ['a', 'b']), /maksimal 1/);
  assert.equal(modifierError([], []), '');
});

test('pembayaran restoran memvalidasi metode dan nominal bilangan sen', () => {
  assert.deepEqual(checkoutPayments('cash', '5000000'), [{ method: 'cash', amount_cents: '5000000' }]);
  assert.throws(() => checkoutPayments('crypto', '100'));
  assert.throws(() => checkoutPayments('cash', '0'));
  assert.throws(() => checkoutPayments('cash', '1.5'));
});

test('alasan pembayaran diblokir mengikuti respons tagihan server', () => {
  assert.match(paymentBlockReason(null), /Memuat/);
  assert.match(paymentBlockReason({ not_sent_count: 2, payment_blocked: true }), /2 item belum dikirim/);
  assert.match(paymentBlockReason({ not_sent_count: 0, payment_blocked: true }), /belum dapat/);
  assert.equal(paymentBlockReason({ not_sent_count: 0, payment_blocked: false }), '');
});

test('label status baris memakai istilah restoran', () => {
  assert.equal(lineStatusLabel('NOT_SENT'), 'Belum dikirim');
  assert.equal(lineStatusLabel('X'), 'X');
});

test('persetujuan pembatalan: belum dikirim bebas, lewat SENT atau bukan manajer wajib PIN', () => {
  assert.equal(cancelNeedsApproval('NOT_SENT', 'cashier'), false);
  assert.equal(cancelNeedsApproval('SENT', 'manager'), false);
  assert.equal(cancelNeedsApproval('SENT', 'cashier'), true);
  assert.equal(cancelNeedsApproval('IN_PROGRESS', 'owner'), true);
});

test('body pembatalan memvalidasi alasan dan pasangan email+PIN', () => {
  assert.deepEqual(cancelBody(' salah pesan '), { reason: 'salah pesan' });
  assert.deepEqual(cancelBody('x', 'a@b.id', '123456'), { reason: 'x', approval: { email: 'a@b.id', pin: '123456' } });
  assert.throws(() => cancelBody('  '));
  assert.throws(() => cancelBody('x'.repeat(201)));
  assert.throws(() => cancelBody('x', 'a@b.id', '12'));
});

test('permintaan ketersediaan menu memakai versi 0 untuk menu baru', () => {
  assert.deepEqual(availabilityRequest('b1', { version: 0 }, true), { body: { branch_id: 'b1', sold_out: true }, version: 0 });
  assert.equal(availabilityRequest('b1', { version: 3 }, false).version, 3);
  assert.throws(() => availabilityRequest('', {}, true));
});

test('label ketersediaan membedakan Habis manual dan stok habis', () => {
  assert.equal(availabilityLabel({ sold_out: true }), 'Habis (manual)');
  assert.equal(availabilityLabel({ sold_out: false, track_stock: true, available_milli: '0' }), 'Stok habis');
  assert.equal(availabilityLabel({ sold_out: false, track_stock: true, available_milli: '2000' }), 'Tersedia');
  assert.equal(availabilityLabel({ sold_out: false, track_stock: false, available_milli: '0' }), 'Tersedia');
});

test('split pembayaran menjaga presisi string sen dan menolak baris kosong', () => {
  assert.deepEqual(splitCheckoutPayments([{ method: 'qris', amount: '60000' }, { method: 'cash', amount: '50000' }]), [{ method: 'qris', amount_cents: '6000000' }, { method: 'cash', amount_cents: '5000000' }]);
  assert.throws(() => splitCheckoutPayments([]));
  assert.throws(() => splitCheckoutPayments([{ method: 'cash', amount: '0' }]));
});

test('edit item mengirim qty dan catatan tanpa menghapus modifier', () => {
  assert.deepEqual(editLineBody('2', ' tanpa gula '), { qty_milli: '2000', note: 'tanpa gula' });
  assert.throws(() => editLineBody('0'));
  assert.throws(() => editLineBody('1', 'x'.repeat(141)));
});

test('diskon restoran memakai helper pricing yang sama dengan kasir retail', () => {
  assert.deepEqual(orderDiscountBody('percent', '12.5', 'promo'), { discount: { kind: 'percent', value: '1250' }, reason: 'promo' });
  assert.equal(orderDiscountBody('amount', '1000').discount.value, '100000');
  assert.equal(orderDiscountBody('none', '').discount.value, '0');
  assert.throws(() => orderDiscountBody('percent', '101'));
});
