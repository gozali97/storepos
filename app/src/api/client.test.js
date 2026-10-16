import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ApiError, API_CODES, createApiClient, newIdempotencyKey } from './client.js';

function jsonResponse(body, { status = 200, requestId = 'rid-test' } = {}) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', 'X-Request-ID': requestId },
  });
}

test('mengirim header konteks bisnis dan CSRF', async () => {
  let captured;
  const api = createApiClient({
    fetchImpl: async (url, init) => {
      captured = { url, init };
      return jsonResponse({ ok: true });
    },
    getBusinessId: () => 'biz-1',
    getCsrfToken: () => 'csrf-1',
  });

  await api.post('/sales', { a: 1 });

  assert.equal(captured.url, '/api/v1/sales');
  assert.equal(captured.init.headers['X-Business-ID'], 'biz-1');
  assert.equal(captured.init.headers['X-CSRF-Token'], 'csrf-1');
  assert.equal(captured.init.credentials, 'include');
});

test('membaca selisih jam dari header Date dan mengabaikan header tidak valid', async () => {
  const offsets = [];
  const serverDate = new Date(Date.now() + 360000).toUTCString();
  const api = createApiClient({ onClockOffset: value => offsets.push(value), fetchImpl: async () => new Response('{}', { headers: { Date: serverDate } }) });
  await api.get('/me');
  assert.equal(offsets.length, 1);
  assert.ok(offsets[0] > 350000 && offsets[0] <= 361000);
  const noDate = createApiClient({ onClockOffset: value => offsets.push(value), fetchImpl: async () => new Response('{}', { headers: { Date: 'invalid' } }) });
  await noDate.get('/me');
  assert.equal(offsets.length, 1);
});

test('meneruskan Idempotency-Key dan If-Match', async () => {
  let init;
  const api = createApiClient({
    fetchImpl: async (url, i) => {
      init = i;
      return jsonResponse({});
    },
  });

  await api.request('POST', '/sales/1/finalize', { body: {}, idempotencyKey: 'key-1', version: 3 });

  assert.equal(init.headers['Idempotency-Key'], 'key-1');
  assert.equal(init.headers['If-Match'], '3');
});

test('memetakan error backend ke ApiError dengan code stabil', async () => {
  const api = createApiClient({
    fetchImpl: async () =>
      jsonResponse({ error: { code: 'STOCK_INSUFFICIENT', message: 'Stok kurang', details: [{ field: 'L1' }] } }, { status: 409 }),
  });

  await assert.rejects(
    () => api.post('/sales/1/finalize', {}),
    (err) => {
      assert.ok(err instanceof ApiError);
      assert.equal(err.code, API_CODES.STOCK_INSUFFICIENT);
      assert.equal(err.status, 409);
      assert.equal(err.requestId, 'rid-test');
      assert.equal(err.details.length, 1);
      return true;
    },
  );
});

test('membungkus kegagalan jaringan', async () => {
  const api = createApiClient({
    fetchImpl: async () => {
      throw new TypeError('failed to fetch');
    },
  });

  await assert.rejects(() => api.get('/products'), (err) => {
    assert.equal(err.code, 'NETWORK');
    assert.match(err.message, /koneksi/i);
    return true;
  });
});

test('204 mengembalikan undefined tanpa parse body', async () => {
  const api = createApiClient({
    fetchImpl: async () => new Response(null, { status: 204 }),
  });
  assert.equal(await api.del('/products/1'), undefined);
});

test('newIdempotencyKey menghasilkan UUID v4 unik', () => {
  const a = newIdempotencyKey();
  const b = newIdempotencyKey();
  assert.notEqual(a, b);
  assert.match(a, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
});

test('401 mencabut sesi UI, tetapi kredensial login salah tidak memicu callback', async () => {
  let expired = 0;
  const api = createApiClient({ onUnauthenticated: () => expired++, fetchImpl: async () => jsonResponse({ error: { code: 'UNAUTHENTICATED' } }, { status: 401 }) });
  await assert.rejects(api.post('/auth/login', { email: 'a', password: 'b' }));
  assert.equal(expired, 0);
  await assert.rejects(api.get('/reports/sales'));
  assert.equal(expired, 1);
});

test('unduhan CSV membawa konteks bisnis dan mempertahankan respons error JSON', async () => {
  let captured;
  const api = createApiClient({
    getBusinessId: () => 'biz-a',
    fetchImpl: async (url, init) => {
      captured = init;
      return new Response('name,amount\nProduk,100\n', { headers: { 'Content-Type': 'text/csv' } });
    },
  });
  const blob = await api.get('/reports/sales?format=csv', { responseType: 'blob' });
  assert.equal(captured.headers['X-Business-ID'], 'biz-a');
  assert.equal(await blob.text(), 'name,amount\nProduk,100\n');
  const forbidden = createApiClient({ fetchImpl: async () => jsonResponse({ error: { code: 'PERMISSION_DENIED', message: 'Tidak boleh' } }, { status: 403 }) });
  await assert.rejects(forbidden.get('/reports/sales?format=csv', { responseType: 'blob' }), error => error.code === 'PERMISSION_DENIED');
});
