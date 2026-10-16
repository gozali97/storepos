import { test } from 'node:test';
import assert from 'node:assert/strict';
import { summarizeSales } from './reporting.js';
test('ringkasan mempertahankan presisi uang dan histori yang tidak lengkap', () => {
  const row = { net_cents: '900719925474099300', discount_cents: '100', returns_cents: '20', sale_count: 1 };
  assert.deepEqual(summarizeSales([row, row]), { net: 1801439850948198600n, discount: 200n, returns: 40n, count: 2n });
  assert.equal(summarizeSales([row, { ...row, net_cents: null }]).net, null);
  assert.deepEqual(summarizeSales([]), { net: 0n, discount: 0n, returns: 0n, count: 0n });
});
