import { test } from 'node:test';
import assert from 'node:assert/strict';
import { nextAction, previousStatus, ticketAgeMinutes, ageTone, activeItems, ticketReady, statusLabel, isBackward } from './kds.js';

const kitchen = { canPrepare: true, canServe: false };
const waiter = { canPrepare: false, canServe: true };

test('aksi satu ketuk maju satu langkah sesuai peran', () => {
  assert.equal(nextAction('SENT', kitchen).next, 'IN_PROGRESS');
  assert.equal(nextAction('IN_PROGRESS', kitchen).next, 'READY');
  assert.equal(nextAction('READY', kitchen), null); // SERVED bukan tugas dapur
  assert.equal(nextAction('READY', waiter).next, 'SERVED');
  assert.equal(nextAction('SENT', waiter), null);
  assert.equal(nextAction('SERVED', waiter), null);
  assert.equal(nextAction('CANCELLED', kitchen), null);
});

test('mundur hanya satu langkah dan tidak dari status awal', () => {
  assert.equal(previousStatus('READY'), 'IN_PROGRESS');
  assert.equal(previousStatus('SENT'), null);
});

test('umur tiket memakai waktu server, bukan jam perangkat', () => {
  const created = '2026-10-14T10:00:00Z';
  const server = '2026-10-14T10:12:30Z';
  assert.equal(ticketAgeMinutes(created, server, 1000, 1000), 12);
  assert.equal(ticketAgeMinutes(created, server, 1000, 1000 + 60000), 13); // satu menit sejak fetch
  assert.equal(ticketAgeMinutes('rusak', server, 0, 0), 0);
  assert.equal(ticketAgeMinutes(server, created, 0, 0), 0); // tidak pernah negatif
});

test('nada umur tiket berubah pada 10 dan 20 menit', () => {
  assert.equal(ageTone(9), 'ok');
  assert.equal(ageTone(10), 'warn');
  assert.equal(ageTone(20), 'danger');
});

test('item disajikan disembunyikan, item dibatalkan tetap tampil', () => {
  const ticket = { items: [{ status: 'SERVED' }, { status: 'CANCELLED' }, { status: 'READY' }] };
  assert.deepEqual(activeItems(ticket).map(i => i.status), ['CANCELLED', 'READY']);
});

test('tiket siap diantar bila semua item aktif READY', () => {
  assert.equal(ticketReady({ items: [{ status: 'READY' }, { status: 'CANCELLED' }] }), true);
  assert.equal(ticketReady({ items: [{ status: 'READY' }, { status: 'IN_PROGRESS' }] }), false);
  assert.equal(ticketReady({ items: [{ status: 'CANCELLED' }] }), false);
});

test('label status memakai istilah dapur', () => {
  assert.equal(statusLabel('IN_PROGRESS'), 'Dimasak');
  assert.equal(statusLabel('X'), 'X');
});

test('mundur dinilai dari urutan, bukan dari nama status tujuan', () => {
  assert.equal(isBackward('SENT', 'IN_PROGRESS'), false); // Mulai masak = maju, tanpa alasan
  assert.equal(isBackward('READY', 'IN_PROGRESS'), true);
  assert.equal(isBackward('IN_PROGRESS', 'SENT'), true);
  assert.equal(isBackward('READY', 'SERVED'), false);
});
import { kitchenOverview } from './kds.js';

test('ringkasan dan filter dapur menghitung baris aktif serta menjaga tiket campuran utuh', () => {
  const tickets = [
    { ticket_id: 'a', items: [{ status: 'SENT' }, { status: 'READY' }, { status: 'SERVED' }] },
    { ticket_id: 'b', items: [{ status: 'IN_PROGRESS' }, { status: 'CANCELLED' }] },
  ];
  const summary = kitchenOverview(tickets, 'READY');
  assert.deepEqual(summary.counts, { SENT: 1, IN_PROGRESS: 1, READY: 1 });
  assert.deepEqual(summary.tickets, [tickets[0]]);
  assert.equal(summary.tickets[0].items.length, 3);
  assert.deepEqual(kitchenOverview([]).counts, { SENT: 0, IN_PROGRESS: 0, READY: 0 });
  assert.deepEqual(kitchenOverview(tickets).tickets, tickets);
});
