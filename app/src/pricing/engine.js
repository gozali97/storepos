// Port mesin harga PRD §13.8 untuk frontend.
// Harus menghasilkan angka IDENTIK dengan backend/internal/pricing untuk setiap
// vektor di backend/internal/pricing/testdata/vectors.json (§13.7).

import { mulQty, roundToMultiple, roundToRupiah } from '../money.js';

/**
 * @typedef {{kind:'none'|'amount'|'percent', value:number}} Discount
 * @typedef {{name:string, rate_bps:number, mode:'exclusive'|'inclusive'}} TaxRule
 * @typedef {{rate_bps:number, include_in_tax_base:boolean}} ServiceChargeRule
 * @typedef {{name:string, price_delta_cents:bigint}} Modifier
 * @typedef {{
 *   id:string,
 *   unit_price_cents:bigint,
 *   qty_milli:bigint,
 *   modifiers?:Modifier[],
 *   line_discount:Discount,
 *   tax?:TaxRule|null,
 *   service_charge?:ServiceChargeRule|null,
 *   discount_eligible:boolean
 * }} Line
 */

/**
 * Terapkan diskon ke basis cents.
 * @param {Discount} d
 * @param {bigint} baseCents
 * @returns {bigint}
 */
function applyDiscount(d, baseCents) {
  if (!d || d.kind === 'none') return 0n;
  if (d.kind === 'amount') {
    const v = BigInt(d.value);
    return v > baseCents ? baseCents : v;
  }
  if (d.kind === 'percent') {
    // base * bps / 10000, half-up ke sen
    const bps = BigInt(d.value);
    const num = baseCents * bps;
    let q = num / 10000n;
    const r = num % 10000n;
    if (r * 2n >= 10000n) q += 1n;
    else if (r * 2n <= -10000n) q -= 1n;
    return q;
  }
  return 0n;
}

/**
 * base * bps / 10000 (half-up ke sen).
 * @param {bigint} base
 * @param {number|bigint} bps
 */
function percentOf(base, bps) {
  const b = BigInt(bps);
  const num = base * b;
  let q = num / 10000n;
  const r = num % 10000n;
  if (r * 2n >= 10000n) q += 1n;
  else if (r * 2n <= -10000n) q -= 1n;
  return q;
}

/**
 * Jalankan mesin harga.
 * @param {{
 *   lines: Line[],
 *   transaction_discount?: Discount,
 *   cash_rounding_multiple?: number|bigint,
 *   apply_cash_rounding?: boolean
 * }} input
 */
export function compute(input) {
  const lines = input.lines || [];
  if (lines.length === 0) throw new Error('pricing: minimal satu baris');

  const results = lines.map(() => ({}));
  let subtotalGross = 0n;
  let totalLineDiscount = 0n;

  // Langkah 1 & 2: gross dan diskon baris.
  for (let i = 0; i < lines.length; i++) {
    const l = lines[i];
    if (l.qty_milli <= 0n) throw new Error(`baris ${i} qty harus > 0`);
    if (l.unit_price_cents < 0n) throw new Error(`baris ${i} harga satuan < 0`);

    let perUnit = l.unit_price_cents;
    const mods = l.modifiers || [];
    for (const m of mods) {
      if (m.price_delta_cents < 0n) throw new Error(`modifier ${m.name} delta < 0`);
      perUnit += m.price_delta_cents;
    }
    const gross = roundToRupiah(mulQty(perUnit, l.qty_milli));
    const lineDisc = roundToRupiah(applyDiscount(l.line_discount, gross));
    if (lineDisc > gross) throw new Error(`diskon baris ${i} melebihi gross`);

    results[i].id = l.id;
    results[i].gross_cents = gross;
    results[i].line_discount_cents = lineDisc;
    results[i].net_before_tx_cents = gross - lineDisc;
    results[i].alloc_discount_cents = 0n;
    subtotalGross += gross;
    totalLineDiscount += lineDisc;
  }

  // Langkah 3: diskon transaksi.
  let eligibleTotal = 0n;
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].discount_eligible) eligibleTotal += results[i].net_before_tx_cents;
  }
  let txDisc = 0n;
  if (eligibleTotal > 0n && input.transaction_discount) {
    txDisc = applyDiscount(input.transaction_discount, eligibleTotal);
    txDisc = roundToRupiah(txDisc);
    if (txDisc > eligibleTotal) txDisc = eligibleTotal;
  }

  if (txDisc > 0n) {
    let allocated = 0n;
    let maxIdx = -1;
    let maxBase = -1n;
    for (let i = 0; i < lines.length; i++) {
      if (!lines[i].discount_eligible) continue;
      const base = results[i].net_before_tx_cents;
      const num = txDisc * base;
      const den = eligibleTotal;
      let alloc = roundToRupiah(num / den);
      const rem = num % den;
      if (rem * 2n >= den) {
        alloc += 100n;
        alloc = roundToRupiah(alloc);
      }
      results[i].alloc_discount_cents = alloc;
      allocated += alloc;
      if (base > maxBase) {
        maxBase = base;
        maxIdx = i;
      }
    }
    const diff = txDisc - allocated;
    if (diff !== 0n && maxIdx >= 0) {
      results[maxIdx].alloc_discount_cents += diff;
    }
  }

  let subtotalNet = 0n;
  for (let i = 0; i < lines.length; i++) {
    results[i].net_cents = results[i].net_before_tx_cents - results[i].alloc_discount_cents;
    subtotalNet += results[i].net_cents;
  }

  // Langkah 5: service charge.
  let totalSC = 0n;
  for (let i = 0; i < lines.length; i++) {
    const sc = lines[i].service_charge;
    if (!sc) {
      results[i].service_charge_cents = 0n;
      continue;
    }
    const base = results[i].net_cents;
    const v = roundToRupiah(percentOf(base, sc.rate_bps));
    results[i].service_charge_cents = v;
    totalSC += v;
  }

  // Langkah 6: pajak.
  let totalTax = 0n;
  for (let i = 0; i < lines.length; i++) {
    const tax = lines[i].tax;
    if (!tax) {
      results[i].tax_cents = 0n;
      results[i].tax_base_cents = 0n;
      continue;
    }
    let base = results[i].net_cents;
    const sc = lines[i].service_charge;
    if (sc && sc.include_in_tax_base) base += results[i].service_charge_cents;

    let taxVal;
    if (tax.mode === 'exclusive') {
      taxVal = percentOf(base, tax.rate_bps);
      results[i].tax_base_cents = base;
    } else {
      // inclusive: tax = base * rate / (10000 + rate)
      const bps = BigInt(tax.rate_bps);
      const num = base * bps;
      const den = 10000n + bps;
      let q = num / den;
      const r = num % den;
      if (r * 2n >= den) q += 1n;
      taxVal = q;
      results[i].tax_base_cents = base - taxVal;
    }
    taxVal = roundToRupiah(taxVal);
    results[i].tax_cents = taxVal;
    totalTax += taxVal;
  }

  // Langkah 7: total sebelum pembulatan tunai.
  let exclusiveTax = 0n;
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].tax && lines[i].tax.mode === 'exclusive') {
      exclusiveTax += results[i].tax_cents;
    }
  }
  const totalBeforeRounding = subtotalNet + totalSC + exclusiveTax;

  // Line total untuk tampilan.
  for (let i = 0; i < lines.length; i++) {
    const exclusive =
      lines[i].tax && lines[i].tax.mode === 'exclusive' ? results[i].tax_cents : 0n;
    results[i].line_total_cents = results[i].net_cents + results[i].service_charge_cents + exclusive;
  }

  // Langkah 8: pembulatan tunai.
  let cashRounding = 0n;
  let total = totalBeforeRounding;
  const mult = input.cash_rounding_multiple ? BigInt(input.cash_rounding_multiple) : 0n;
  if (input.apply_cash_rounding && mult > 0n) {
    const rounded = roundToMultiple(totalBeforeRounding, mult);
    cashRounding = rounded - totalBeforeRounding;
    total = rounded;
  }

  return {
    lines: results,
    subtotal_gross_cents: subtotalGross,
    total_line_discount_cents: totalLineDiscount,
    transaction_discount_cents: txDisc,
    subtotal_net_cents: subtotalNet,
    service_charge_cents: totalSC,
    tax_cents: totalTax,
    total_before_rounding_cents: totalBeforeRounding,
    cash_rounding_cents: cashRounding,
    total_cents: total,
  };
}
