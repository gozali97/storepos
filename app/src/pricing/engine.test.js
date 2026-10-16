// Uji vektor bersama antara backend Go dan frontend JS.
// Membaca ../../backend/internal/pricing/testdata/vectors.json dan memverifikasi
// bahwa compute() menghasilkan angka IDENTIK (§13.7).

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

import { compute } from './engine.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const vectorsPath = resolve(__dirname, '../../../backend/internal/pricing/testdata/vectors.json');

const raw = await readFile(vectorsPath, 'utf8');
const cases = JSON.parse(raw);

for (const c of cases) {
  test(c.name, () => {
    const input = buildInput(c.input);
    const got = compute(input);

    assertEq('subtotal_gross_cents', got.subtotal_gross_cents, c.expected.subtotal_gross_cents);
    assertEq('total_line_discount_cents', got.total_line_discount_cents, c.expected.total_line_discount_cents);
    assertEq('transaction_discount_cents', got.transaction_discount_cents, c.expected.transaction_discount_cents);
    assertEq('subtotal_net_cents', got.subtotal_net_cents, c.expected.subtotal_net_cents);
    assertEq('service_charge_cents', got.service_charge_cents, c.expected.service_charge_cents);
    assertEq('tax_cents', got.tax_cents, c.expected.tax_cents);
    assertEq('total_before_rounding_cents', got.total_before_rounding_cents, c.expected.total_before_rounding_cents);
    assertEq('cash_rounding_cents', got.cash_rounding_cents, c.expected.cash_rounding_cents);
    assertEq('total_cents', got.total_cents, c.expected.total_cents);
  });
}

function assertEq(name, got, wantStr) {
  const want = BigInt(wantStr);
  assert.equal(got, want, `${name}: got ${got}, want ${want}`);
}

function buildInput(v) {
  return {
    lines: v.lines.map((l) => ({
      id: l.id,
      unit_price_cents: BigInt(l.unit_price_cents),
      qty_milli: BigInt(l.qty_milli),
      modifiers: (l.modifiers || []).map((m) => ({
        name: m.name,
        price_delta_cents: BigInt(m.price_delta_cents),
      })),
      line_discount: l.line_discount,
      tax: l.tax,
      service_charge: l.service_charge,
      discount_eligible: !!l.discount_eligible,
    })),
    transaction_discount: v.transaction_discount,
    cash_rounding_multiple: v.cash_rounding_multiple,
    apply_cash_rounding: v.apply_cash_rounding,
  };
}
