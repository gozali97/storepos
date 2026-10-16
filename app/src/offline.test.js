import { test } from 'node:test';
import assert from 'node:assert/strict';
import { offlineCart, syncOfflineCart } from './offline.js';
import { addProduct, emptyCart } from './cashier.js';

const cart = addProduct(emptyCart(), { variant: { id: 'variant-a' } });
test('keranjang offline mempertahankan versi draft asal dan hanya menyimpan data keranjang/katalog', () => {
  const active = { id: 'draft-a', version: 3, cart: emptyCart(), status: 'active' };
  const data = offlineCart({ active, catalog: [], productUnits: {}, approval: { pin: '123456' } });
  assert.equal(data.base.version, 3);
  assert.equal(data.approval, undefined);
  assert.deepEqual(offlineCart({}).cart, emptyCart());
});
test('sinkronisasi memakai versi asal dan menolak perubahan server', async () => {
  const base = { id: 'draft-a', version: 3, status: 'active', cart: emptyCart() };
  let captured;
  const client = { get: async () => ({ drafts: [base] }), put: async (...args) => { captured = args; return { ...base, version: 4, cart }; } };
  const data = { base, cart };
  await syncOfflineCart(client, data, 'branch-a', 'register-a');
  assert.deepEqual(captured, ['/drafts/draft-a', { status: 'active', cart }, { version: 3 }]);
  captured = null;
  client.get = async () => ({ drafts: [{ ...base, version: 4 }] });
  await assert.rejects(() => syncOfflineCart(client, data, 'branch-a', 'register-a'), /berubah/);
  assert.equal(captured, null);
  assert.equal(data.cart.lines.length, 1);
});
test('respons sinkronisasi terputus dapat direkonsiliasi tanpa draft ganda', async () => {
  let posts = 0;
  const draft = { id: 'draft-a', version: 1, status: 'active', cart };
  const client = { get: async () => ({ drafts: [draft] }), post: async () => { posts++; } };
  assert.equal(await syncOfflineCart(client, { base: null, cart }, 'branch-a', 'register-a'), draft);
  assert.equal(await syncOfflineCart(client, { base: { id: 'draft-a', version: 0 }, cart }, 'branch-a', 'register-a'), draft);
  assert.equal(posts, 0);
  client.get = async () => ({ drafts: [] });
  client.post = async (path, body) => { assert.equal(path, '/drafts'); assert.equal(body.register_id, 'register-a'); return draft; };
  assert.equal(await syncOfflineCart(client, { base: null, cart }, 'branch-a', 'register-a'), draft);
});
