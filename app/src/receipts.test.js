import { test } from 'node:test';
import assert from 'node:assert/strict';
import { printCopy } from './receipts.js';

test('dialog cetak hanya dibuka setelah audit cetak ulang berhasil', async () => {
  const events = [];
  await printCopy({ post: async (path, body) => { assert.deepEqual(body, {}); events.push(path); } }, 'sale-a', () => events.push('print'));
  assert.deepEqual(events, ['/sales/sale-a/reprints', 'print']);
  await assert.rejects(() => printCopy({ post: async () => { throw new Error('Akses ditolak'); } }, 'sale-a', () => events.push('print')));
  assert.equal(events.length, 2);
});
