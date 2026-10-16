import { test } from 'node:test';
import assert from 'node:assert/strict';
import { loadPendingMutation, performPendingMutation, isUncertainMutation } from './pendingMutation.js';

function storage() {
  const m = new Map();
  return { getItem: k => m.get(k) ?? null, setItem: (k,v) => m.set(k,v), removeItem: k => m.delete(k) };
}
const operation = { path: '/restaurant/orders/o/checkout', body: { payments: [{ method: 'cash', amount_cents: '10000' }] }, version: 3, key: 'original-key', createdAt: Date.now() };

test('timeout preserves exact operation across refresh and retry then clears on success', async () => {
  const s = storage(); const calls = [];
  const client = { async post(path,body,options) { calls.push(JSON.parse(JSON.stringify({path,body,options}))); if(calls.length === 1) throw Object.assign(new Error('timeout'), { code: 'NETWORK' }); return { sale_id: 'sale' }; } };
  const mutable = structuredClone(operation);
  await assert.rejects(performPendingMutation(client,s,'owner',mutable));
  mutable.version = 99; mutable.body.payments[0].amount_cents = '90000';
  const restored = loadPendingMutation(s,'owner');
  assert.deepEqual(restored,operation);
  assert.equal(loadPendingMutation(s,'other-user'),null);
  assert.deepEqual(await performPendingMutation(client,s,'owner',restored),{sale_id:'sale'});
  assert.deepEqual(calls[0],calls[1]);
  assert.equal(loadPendingMutation(s,'owner'),null);
});

test('definite validation error clears pending, uncertain errors retain it', async () => {
  for(const error of [{status:422},{status:500},{status:408},{status:429},{status:409,code:'IDEMPOTENCY_CONFLICT'}]) {
    const s = storage();
    await assert.rejects(performPendingMutation({post:async()=>{throw error;}},s,'key',operation));
    assert.equal(!!loadPendingMutation(s,'key'),isUncertainMutation(error));
  }
});

test('storage failure prevents sending a stock or payment action', async () => {
  let sent=false;
  const s=storage();s.setItem=()=>{throw new Error('quota');};
  await assert.rejects(performPendingMutation({post:async()=>{sent=true;}},s,'key',operation));
  assert.equal(sent,false);
});

test('corrupt recovery data fails closed instead of silently losing pending action', () => {
  const s=storage();s.setItem('key','broken-json');
  assert.throws(()=>loadPendingMutation(s,'key'));
  s.setItem('key',JSON.stringify({path:'/x'}));
  assert.throws(()=>loadPendingMutation(s,'key'));
});

test('expired recovery cannot send a request after backend retention window', async () => {
  const s=storage(); let sent=false;
  const expired={...operation,createdAt:Date.now()-73*60*60*1000};
  await assert.rejects(performPendingMutation({post:async()=>{sent=true;}},s,'key',expired));
  assert.equal(sent,false);
  assert.throws(()=>loadPendingMutation(s,'key'),/72 jam/);
});

test('kitchen-send retry preserves absent body and original version', async () => {
  const s=storage();const op={path:'/restaurant/orders/o/send',key:'send-key',version:4};let call;
  await performPendingMutation({post:async(...args)=>{call=args;}},s,'key',op);
  assert.deepEqual(call,[op.path,undefined,{version:4,idempotencyKey:'send-key'}]);
});
