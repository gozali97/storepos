// Utilitas uang & kuantitas untuk frontend StorePOS.
// Mengikuti PRD §13.7: uang disimpan sebagai BigInt sen, kuantitas sebagai BigInt milli-unit.
// JSON ↔ domain memakai string integer agar tidak kehilangan presisi.

/** @typedef {bigint} Cents */
/** @typedef {bigint} Milli */

/**
 * Format Cents sebagai "Rp 1.234.567" atau "Rp 1.234,56" bila ada pecahan sen.
 * @param {Cents} c
 */
export function formatIDR(c) {
  if (typeof c !== 'bigint') throw new TypeError('formatIDR butuh BigInt');
  const neg = c < 0n;
  let v = neg ? -c : c;
  const rupiah = v / 100n;
  const sen = v % 100n;
  let s = withDots(rupiah.toString());
  if (sen !== 0n) {
    s = `${s},${sen.toString().padStart(2, '0')}`;
  }
  return `${neg ? '-' : ''}Rp ${s}`;
}

function withDots(n) {
  if (n.length <= 3) return n;
  const parts = [];
  let i = n.length;
  while (i > 3) {
    parts.unshift(n.slice(i - 3, i));
    i -= 3;
  }
  parts.unshift(n.slice(0, i));
  return parts.join('.');
}

/** @param {Cents} c */
export function toCentsString(c) {
  return typeof c === 'bigint' ? c.toString() : String(c);
}

/** @returns {Cents} */
export function parseCents(s) {
  if (typeof s === 'bigint') return s;
  if (typeof s === 'number') {
    if (!Number.isInteger(s)) throw new TypeError('parseCents: number harus integer');
    return BigInt(s);
  }
  if (typeof s !== 'string') throw new TypeError('parseCents: butuh string');
  return BigInt(s.trim());
}

/** @returns {Milli} */
export function parseMilli(s) {
  if (typeof s === 'bigint') return s;
  if (typeof s === 'number') return BigInt(s);
  return BigInt(String(s).trim());
}

/** @param {Milli} q */
export function formatQty(q) {
  if (typeof q !== 'bigint') throw new TypeError('formatQty butuh BigInt');
  const neg = q < 0n;
  let v = neg ? -q : q;
  const whole = v / 1000n;
  const frac = v % 1000n;
  if (frac === 0n) return `${neg ? '-' : ''}${whole.toString()}`;
  let fs = frac.toString().padStart(3, '0').replace(/0+$/, '');
  return `${neg ? '-' : ''}${whole.toString()},${fs}`;
}

/**
 * Half-up ke rupiah penuh (100 sen) sesuai §13.8 default.
 * @param {Cents} c
 * @returns {Cents}
 */
export function roundToRupiah(c) {
  return roundToMultiple(c, 100n);
}

/**
 * Pembulatan ke kelipatan sen tertentu, half-up.
 * @param {Cents} c
 * @param {bigint} multiple sen
 */
export function roundToMultiple(c, multiple) {
  if (typeof c !== 'bigint' || typeof multiple !== 'bigint') {
    throw new TypeError('roundToMultiple butuh BigInt');
  }
  if (multiple <= 0n) return c;
  if (multiple === 1n) return c;
  let q = c / multiple;
  let r = c % multiple;
  if (r === 0n) return q * multiple;
  const absR = r < 0n ? -r : r;
  if (absR * 2n >= multiple) {
    q += r > 0n ? 1n : -1n;
  }
  return q * multiple;
}

/**
 * Hitung price × qty dengan konversi milli-unit. Belum dibulatkan.
 * @param {Cents} price
 * @param {Milli} qty
 * @returns {Cents}
 */
export function mulQty(price, qty) {
  return (price * qty) / 1000n;
}
