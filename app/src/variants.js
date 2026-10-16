import { rupiahToCents } from './forms.js';

export function productVariants(variants, values) {
  if (!variants.length || variants.length > 100) throw new Error('Produk harus memiliki 1 sampai 100 varian.');
  const skus = variants.map(variant => variant.sku.trim().toLowerCase());
  if (skus.some(sku => !sku) || new Set(skus).size !== skus.length) throw new Error('SKU tiap varian harus diisi dan berbeda.');
  return variants.map(variant => ({
    name: variant.name, sku: variant.sku.trim(), attributes: variant.attributes,
    base_unit: values.base_unit.trim(), price_cents: rupiahToCents(variant.price),
    allow_fractional: values.fractional === 'on', price_includes_tax: values.includes_tax === 'on', barcodes: [],
  }));
}
