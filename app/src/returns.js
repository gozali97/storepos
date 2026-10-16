import { quantityToMilli } from './cashier.js';

export function returnLines(lines, entries) {
  const result = [];
  for (const line of lines) {
    const entry = entries[line.sale_line_id];
    if (!entry?.selected) continue;
    const qty = quantityToMilli(entry.qty);
    if (BigInt(qty) > BigInt(line.remaining_qty_milli)) throw new Error(`Jumlah retur ${line.product_name} melebihi sisa yang dapat diretur.`);
    if (!line.allow_fractional && BigInt(qty) % 1000n !== 0n) throw new Error(`Jumlah retur ${line.product_name} harus bulat.`);
    if (!['restock', 'damaged'].includes(entry.condition)) throw new Error('Kondisi barang retur tidak valid.');
    result.push({ sale_line_id: line.sale_line_id, qty_milli: qty, condition: entry.condition });
  }
  if (!result.length) throw new Error('Pilih minimal satu barang untuk diretur.');
  return result;
}

const METHOD_LABELS = { cash: 'tunai', qris: 'QRIS', card: 'kartu', transfer: 'transfer' };

/** Rincian refund per metode asal; bagian nontunai ditandai manual karena belum terintegrasi. */
export function refundSummary(refunds = []) {
  return refunds.map(part => ({
    label: METHOD_LABELS[part.method] ?? part.method,
    cents: part.amount_cents,
    manual: !!part.manual_refund_required,
  }));
}
