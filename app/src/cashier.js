import { compute } from './pricing/engine.js';
import { rupiahToCents } from './forms.js';

export const emptyCart = () => ({
  lines: [],
  note: '',
  discount: { kind: 'none', value: '0' },
  discount_reason: '',
});

export function normalizeCart(cart) {
  return {
    ...emptyCart(),
    ...cart,
    lines: (cart?.lines ?? []).map(line => ({
      unit_id: '',
      note: '',
      discount: { kind: 'none', value: '0' },
      ...line,
    })),
  };
}

export function quantityToMilli(value, allowZero = false) {
  const match = /^(\d+)(?:[.,](\d{1,3}))?$/.exec(String(value).trim());
  if (!match) throw new Error('Kuantitas harus positif dengan maksimal tiga desimal.');
  const milli = BigInt(match[1]) * 1000n + BigInt((match[2] ?? '').padEnd(3, '0'));
  if (milli < 0n || (!allowZero && milli === 0n)) throw new Error(allowZero ? 'Kuantitas tidak boleh negatif.' : 'Kuantitas harus lebih dari nol.');
  return milli.toString();
}

export function isDuplicateScan(previous, value, at, enabled = true) {
  const elapsed = at - previous.at;
  return enabled && previous.value === value && elapsed >= 0 && elapsed < 300;
}

export function isScannerInput(times, enterAt) {
  return times.length >= 4 && enterAt >= times.at(-1) && enterAt - times.at(-1) <= 50
    && times.every((time, index) => index === 0 || time >= times[index - 1] && time - times[index - 1] <= 35);
}

export function discountFromInput(kind, value) {
  if (kind === 'none') return { kind: 'none', value: '0' };
  if (kind === 'amount') {
    const cents = rupiahToCents(value);
    if (BigInt(cents) <= 0n) throw new Error('Diskon nominal harus lebih dari nol.');
    return { kind, value: cents };
  }
  if (kind === 'percent') {
    const match = /^(\d+)(?:[.,](\d{1,2}))?$/.exec(String(value).trim());
    if (!match) throw new Error('Diskon persen harus memiliki maksimal dua desimal.');
    const bps = BigInt(match[1]) * 100n + BigInt((match[2] ?? '').padEnd(2, '0'));
    if (bps <= 0n || bps > 10000n) throw new Error('Diskon persen harus lebih dari 0 dan maksimal 100.');
    return { kind, value: bps.toString() };
  }
  throw new Error('Jenis diskon tidak valid.');
}

export function discountToInput(discount) {
  if (discount?.kind === 'amount') {
    const cents = BigInt(discount.value);
    const fraction = String(cents % 100n).padStart(2, '0').replace(/0+$/, '');
    return `${cents / 100n}${fraction ? `.${fraction}` : ''}`;
  }
  if (discount?.kind === 'percent') {
    const bps = BigInt(discount.value);
    const fraction = String(bps % 100n).padStart(2, '0').replace(/0+$/, '');
    return `${bps / 100n}${fraction ? `.${fraction}` : ''}`;
  }
  return '';
}

export function addProduct(cart, hit, unit = null) {
  if (unit && unit.sale_price_cents == null) throw new Error('Harga jual satuan belum diatur.');
  const unitID = unit?.unit_id ?? '';
  const next = normalizeCart(cart);
  const existing = next.lines.find(line => line.variant_id === hit.variant.id && (line.unit_id || '') === unitID);
  if (existing) existing.qty_milli = (BigInt(existing.qty_milli) + 1000n).toString();
  else next.lines.push({
    variant_id: hit.variant.id,
    unit_id: unitID,
    qty_milli: '1000',
    note: '',
    discount: { kind: 'none', value: '0' },
  });
  return next;
}

export function linePrice(line, productsByID, unitsByID = new Map()) {
  if (line.unit_id) return unitsByID.get(line.unit_id)?.sale_price_cents ?? null;
  return productsByID.get(line.variant_id)?.variant.price_cents ?? null;
}

export function cartTotals(cart, productsByID, unitsByID = new Map()) {
  if (!cart.lines.length) return { subtotal_gross_cents: 0n, total_line_discount_cents: 0n, transaction_discount_cents: 0n, subtotal_net_cents: 0n };
  const lines = [];
  for (const line of cart.lines) {
    const price = linePrice(line, productsByID, unitsByID);
    if (price == null) return null;
    lines.push({
      id: String(lines.length),
      unit_price_cents: BigInt(price),
      qty_milli: BigInt(line.qty_milli),
      line_discount: line.discount,
      discount_eligible: true,
    });
  }
  return compute({ lines, transaction_discount: cart.discount });
}

export function cartSubtotal(cart, productsByID, unitsByID = new Map()) {
  return cartTotals(cart, productsByID, unitsByID)?.subtotal_net_cents ?? null;
}

export function reviewCart(cart, oldProducts, oldUnits, newProducts, newUnits) {
  const unavailable = [], changed = [];
  for (const line of cart.lines) {
    const product = newProducts.get(line.variant_id);
    const label = oldProducts.get(line.variant_id)?.product_name ?? line.variant_id;
    const price = linePrice(line, newProducts, newUnits);
    if (!product || price == null) { unavailable.push(label); continue; }
    const previousPrice = linePrice(line, oldProducts, oldUnits);
    if (previousPrice == null || BigInt(previousPrice) !== BigInt(price)) changed.push(label);
  }
  return { unavailable: [...new Set(unavailable)], changed: [...new Set(changed)] };
}

export function finalizeBody(draft, branchID, registerID, payments, customerID = '') {
  return {
    draft_id: draft.id,
    draft_version: draft.version,
    branch_id: branchID,
    register_id: registerID,
    ...(customerID ? { customer_id: customerID } : {}),
    lines: draft.cart.lines.map(line => ({
      variant_id: line.variant_id,
      ...(line.unit_id ? { unit_id: line.unit_id } : {}),
      qty_milli: line.qty_milli,
      discount: line.discount,
    })),
    discount: draft.cart.discount,
    discount_reason: draft.cart.discount_reason,
    payments,
  };
}

export function optionalApproval(email, pin) {
  const normalizedEmail = String(email ?? '').trim();
  const normalizedPIN = String(pin ?? '').trim();
  if (!normalizedEmail && !normalizedPIN) return undefined;
  if (!normalizedEmail || !/^\d{6}$/.test(normalizedPIN)) {
    throw new Error('Email atasan dan PIN 6 digit harus diisi bersama.');
  }
  return { email: normalizedEmail, pin: normalizedPIN };
}
