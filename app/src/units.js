export function resolveDefaultUnit(units, defaults, kind) {
  if (!['purchase', 'sale'].includes(kind)) throw new Error('Jenis default satuan tidak valid.');
  const id = defaults?.[`default_${kind}_unit_id`];
  if (!id) return null;
  const unit = units.find(item => item.unit_id === id && !item.archived);
  if (!unit || (kind === 'sale' && unit.sale_price_cents == null)) {
    throw new Error('Satuan default sudah berubah/tidak tersedia. Atur ulang default sebelum melanjutkan.');
  }
  return unit;
}
