import { quantityToMilli, discountToInput } from './cashier.js';
import { rupiahToCents } from './forms.js';
import { formatQty } from './money.js';

export function receiptDraftBody(values, lines) {
  if (!lines.length || lines.length > 500) throw new Error('Penerimaan harus memiliki 1–500 baris.');
  return {
    received_date: values.received_date,
    invoice_number: String(values.invoice_number ?? '').trim(),
    supplier_id: values.supplier_id || null,
    lines: lines.map(line => {
      if (!line.unit_id) throw new Error('Pilih satuan pembelian setiap baris.');
      return { unit_id: line.unit_id, qty_milli: quantityToMilli(line.qty), purchase_price_cents: rupiahToCents(line.price) };
    }),
  };
}

export function receiptEditableLines(receipt) {
  return receipt.lines.map(line => ({
    variant_id: line.variant_id, unit_id: line.unit_id,
    qty: formatQty(BigInt(line.qty_milli)).replace(',', '.'),
    price: discountToInput({ kind: 'amount', value: line.purchase_price_cents }),
  }));
}
