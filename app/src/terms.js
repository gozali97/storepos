// Kamus istilah per modul usaha (PRD §5.1).
//
// Nama entitas di backend tidak berubah; hanya label antarmuka yang menyesuaikan.
// Contoh: "Produk" ditampilkan sebagai "Menu" di restoran dan "Obat/Produk" di apotek.

export const DEFAULT_TERMS = {
  product: 'Produk',
  products: 'Produk',
  productPlural: 'produk',
  category: 'Kategori',
  variant: 'Varian',
  tax: 'Pajak',
  table: 'Meja',
  kitchen: 'Dapur',
  order: 'Pesanan',
  stock: 'Stok',
  expiry: 'Tanggal kedaluwarsa',
  batch: 'Batch',
};

const OVERRIDES = {
  retail: {},
  restaurant: {
    product: 'Menu',
    products: 'Menu',
    productPlural: 'menu',
    table: 'Meja',
    order: 'Pesanan',
    kitchen: 'Dapur',
  },
  pharmacy: {
    product: 'Obat/Produk',
    products: 'Obat/Produk',
    productPlural: 'obat/produk',
    batch: 'Batch',
    expiry: 'Tanggal kedaluwarsa (ED)',
  },
};

/** Daftar jenis usaha yang dikenali backend. */
export const BUSINESS_TYPES = ['retail', 'restaurant', 'pharmacy'];

/**
 * Mengembalikan istilah untuk jenis usaha tertentu.
 * @param {'retail'|'restaurant'|'pharmacy'} businessType
 */
export function termsFor(businessType) {
  const override = OVERRIDES[businessType];
  if (!override) return { ...DEFAULT_TERMS };
  return { ...DEFAULT_TERMS, ...override };
}

/** Label manusiawi untuk jenis usaha (dipakai di pemilih bisnis/onboarding). */
export const BUSINESS_TYPE_LABELS = {
  retail: 'Toko/Minimarket',
  restaurant: 'Kafe & Restoran',
  pharmacy: 'Apotek',
};

/** Modul yang aktif berdasarkan jenis usaha (PRD CORE-002). */
export const MODULES_FOR_TYPE = {
  retail: ['retail'],
  restaurant: ['restaurant'],
  pharmacy: ['pharmacy'],
};
