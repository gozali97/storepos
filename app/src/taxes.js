import { rupiahToCents } from './forms.js';

// Persen -> basis point via integer string; Number hanya sesudah nilai dibatasi 0..10000.
export function taxRuleBody(name, rate, mode) {
  const text = String(name ?? '').trim();
  if (!text || [...text].length > 100) throw new Error('Nama pajak wajib, maksimal 100 karakter.');
  const bps = BigInt(rupiahToCents(rate));
  if (bps > 10000n) throw new Error('Tarif pajak harus 0–100%.');
  if (!['inclusive', 'exclusive'].includes(mode)) throw new Error('Mode pajak tidak valid.');
  return { name: text, rate_bps: Number(bps), mode };
}
