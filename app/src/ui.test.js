import { test } from 'node:test';
import assert from 'node:assert/strict';
import { build } from 'esbuild';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const fixture = path.join(root, 'test/ui-fixtures.jsx');
const result = await build({
  stdin: { contents: `export { LoginPage } from './src/pages/LoginPage.jsx'; export { AdminLayout } from './src/components/AdminLayout.jsx'; export { default as HomePage } from './src/pages/HomePage.jsx'; export { default as ReportsPage } from './src/pages/ReportsPage.jsx'; export { default as CashierPage } from './src/pages/CashierPage.jsx'; export { default as SettingsPage } from './src/pages/SettingsPage.jsx'; export { default as MembersPage } from './src/pages/MembersPage.jsx'; export { default as ProductUnits } from './src/pages/ProductUnits.jsx'; export { NumericKeypad } from './src/components/NumericKeypad.jsx'; export { default as SalesPage } from './src/pages/SalesPage.jsx'; export { ReceiptPaper } from './src/components/Receipt.jsx'; export { default as VariantBuilder } from './src/pages/VariantBuilder.jsx'; export { default as OfflineCashier } from './src/pages/OfflineCashier.jsx'; export { default as OnboardingPage } from './src/pages/OnboardingPage.jsx';`, resolveDir: root, loader: 'jsx' },
  bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic', external: ['react', 'react-dom', 'react-router-dom'],
  plugins: [{ name: 'ui-fixture', setup(builder) { builder.onResolve({ filter: /(?:SessionContext|WorkspaceContext)\.jsx$|useResource\.js$/ }, () => ({ path: fixture })); } }],
});
const require = createRequire(import.meta.url);
const module = { exports: {} };
new Function('require', 'module', 'exports', result.outputFiles[0].text)(require, module, module.exports);
const { createElement } = require('react');
const { renderToStaticMarkup } = require('react-dom/server');
const { StaticRouter } = require('react-router-dom');
const { LoginPage, AdminLayout, HomePage, ReportsPage, CashierPage, SettingsPage, MembersPage, ProductUnits, NumericKeypad, SalesPage, ReceiptPaper, VariantBuilder, OfflineCashier, OnboardingPage } = module.exports;
const branch = { id: 'branch-a', name: 'Cabang uji', timezone: 'Asia/Jakarta' };
function reset() {
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: { onLine: true } });
  globalThis.__storeposTest = {
    session: { status: 'tamu', businessId: 'biz-a', user: { email: 'akun@example.test' }, business: { business_id: 'biz-a', business_name: 'Bisnis uji', business_type: 'retail', role: 'owner' }, memberships: [{ business_id: 'biz-a', business_name: 'Bisnis uji' }] },
    workspace: { branch, register: { id: 'reg-a', name: 'Terminal uji' }, branches: [branch], registers: [], loading: false },
    resource: () => ({ data: null, loading: false, error: null, reload() {} }),
  };
}
function render(component, props = {}) { return renderToStaticMarkup(createElement(StaticRouter, { location: '/' }, createElement(component, props))); }

test('login memiliki input berlabel, validasi native, dan kontrol sandi', () => {
  reset();
  const html = render(LoginPage);
  assert.match(html, /for="email"/);
  assert.match(html, /type="email"[^>]*required/);
  assert.match(html, /autoComplete="current-password"/i);
  assert.match(html, /Tampilkan kata sandi/);
  assert.doesNotMatch(html, /noValidate/);
});
test('menu mengikuti peran dan memakai navigasi yang benar', () => {
  reset();
  let html = render(AdminLayout);
  assert.match(html, /href="\/laporan"/);
  assert.match(html, /href="\/audit"/);
  assert.match(html, /href="\/anggota"/);
  globalThis.__storeposTest.session.business.role = 'cashier';
  html = render(AdminLayout);
  assert.doesNotMatch(html, /href="\/laporan"|href="\/audit"|href="\/supplier"|href="\/anggota"/);
  assert.match(html, /href="\/kasir"/);
  assert.match(html, /href="\/pengeluaran"/);
  assert.match(html, /Lewati ke konten/);
  assert.match(html, /aria-label="Cabang aktif"/);
  assert.match(html, /Tanpa koneksi|Online/);
});
test('layar kasir memiliki scan, shift, diskon, catatan, dan total server-aware', () => {
  reset();
  const html = render(CashierPage);
  assert.match(html, /Cari atau scan produk/);
  assert.match(html, /Buka shift/);
  assert.match(html, /Keranjang/);
  assert.match(html, /Diskon &amp; catatan transaksi/);
  assert.match(html, /F6/);
  assert.match(html, /aria-label="Rincian transaksi"/);
  assert.match(html, /<dt>Subtotal<\/dt><dd>Rp 0<\/dd>/);
  assert.match(html, /<dt>Diskon<\/dt><dd>Rp 0<\/dd>/);
  assert.match(html, /Total akhir \(estimasi\)/);
  assert.match(html, /Belum termasuk pajak/);
  assert.match(html, /Abaikan scan ganda di bawah 300 ms/);
  assert.match(html, /Bunyi sukses \/ gagal/);
});
test('dashboard tanpa data tidak mengarang angka, grafik, atau performa', () => {
  reset();
  const html = render(HomePage);
  assert.match(html, /Transaksi yang selesai/);
  assert.doesNotMatch(html, /Rp 0|\+55%|53,000/);
});
test('panduan awal memiliki lima langkah, satu langkah aktif dan status pemuatan', () => {
  reset();
  const html = render(OnboardingPage);
  assert.match(html, /aria-label="Langkah pengaturan"/);
  assert.equal((html.match(/class="setup-step-number"/g) ?? []).length, 5);
  assert.equal((html.match(/aria-current="step"/g) ?? []).length, 1);
  assert.match(html, /aria-busy="true"/);
  assert.match(html, /Memuat pengaturan/);
  assert.match(html, /href="\/dashboard"/);
  globalThis.__storeposTest.session.business.role = 'cashier';
  const restricted = render(OnboardingPage);
  assert.match(restricted, /hanya untuk Pemilik/);
  assert.doesNotMatch(restricted, /setup-layout/);
});
test('dashboard menampilkan konteks cabang, periode dan pintasan sesuai peran usaha', () => {
  reset();
  globalThis.__storeposTest.session.business.business_type = 'restaurant';
  let html = render(HomePage);
  assert.match(html, /Ringkasan 7 hari/);
  assert.match(html, /Cabang uji/);
  assert.match(html, /Terminal uji/);
  assert.match(html, /aria-label="Akses cepat"/);
  assert.match(html, /href="\/meja"/);
  assert.match(html, /href="\/kasir"/);
  globalThis.__storeposTest.session.business.role = 'kitchen';
  html = render(HomePage);
  assert.match(html, /href="\/dapur"/);
  assert.doesNotMatch(html, /href="\/kasir"|href="\/meja"|href="\/laporan"/);
  globalThis.__storeposTest.session.business = { business_type: 'pharmacy', role: 'warehouse' };
  html = render(HomePage);
  assert.match(html, /href="\/batch"/);
  assert.doesNotMatch(html, /href="\/kasir"|href="\/dapur"|href="\/laporan"/);
});
test('laporan menampilkan penolakan izin dan menyembunyikan laba tanpa izin', () => {
  reset();
  globalThis.__storeposTest.resource = () => ({ data: null, loading: false, error: { status: 403, message: 'Izin laporan tidak tersedia.' } });
  assert.match(render(ReportsPage), /Akses dibatasi/);
  globalThis.__storeposTest.resource = () => ({ data: { rows: [] }, loading: true, error: null });
  assert.match(render(ReportsPage), /Memuat data/);
});
test('halaman stok menyediakan operasi gudang dan laporan saldo', () => {
  reset();
  const html = render(ReportsPage, { stockOnly: true });
  assert.match(html, /Operasi persediaan/);
  assert.match(html, /Penerimaan/);
  assert.match(html, /Penyesuaian/);
  assert.match(html, /Kartu stok/);
  assert.match(html, /Stok minimum/);
  assert.match(html, /Opname/);
  assert.match(html, /Ekspor CSV/);
  assert.match(html, /Ekspor latar belakang/);
});

test('pengaturan memakai nilai server dan hanya Pemilik melihat mutasi organisasi', () => {
  reset();
  globalThis.__storeposTest.resource = path => ({
    data: path === '/settings/operational-policy'
      ? { version: 3, policy: { cashier_discount_bps: 1250, cash_out_approval_cents: '75000000', variance_approval_cents: '2500000', cash_rounding_enabled: true, cash_rounding_multiple_cents: '10000' } }
      : path.startsWith('/branches/') ? { branch_id: 'branch-a', code: 'UTAMA', name: 'Cabang uji', timezone: 'Asia/Jakarta', address: 'Alamat tersimpan', receipt_header: 'Header tersimpan', receipt_footer: 'Footer tersimpan', version: 2 }
      : { register_id: 'reg-a', name: 'Terminal uji', status: 'active', version: 4 },
    loading: false, error: null, reload() {},
  });
  const html = render(SettingsPage);
  assert.match(html, /Kebijakan operasional/);
  assert.match(html, /value="12.5"/);
  assert.match(html, /value="750000"/);
  assert.match(html, /Alamat tersimpan/);
  assert.match(html, /Header tersimpan/);
  assert.match(html, /id="edit_code"[^>]*readOnly/i);
  assert.match(html, /Tambah terminal/);
  globalThis.__storeposTest.session.business.role = 'manager';
  assert.doesNotMatch(render(SettingsPage), /Kebijakan operasional|Tambah cabang|Nonaktifkan terminal/);
});

test('Manajer tidak dapat memilih peran Pemilik/Manajer pada halaman anggota', () => {
  reset();
  globalThis.__storeposTest.session.business.role = 'manager';
  globalThis.__storeposTest.resource = () => ({ data: { memberships: [{ membership_id: 'member-a', user_id: 'user-a', role: 'cashier', status: 'active', branch_ids: ['branch-a'], version: 1 }] }, loading: false, error: null, reload() {} });
  const html = render(MembersPage);
  assert.match(html, /user-a/);
  assert.match(html, /Undang anggota/);
  assert.match(html, /Cabang uji/);
  assert.doesNotMatch(html, /option[^>]*value="owner"|option[^>]*value="manager"/);
  globalThis.__storeposTest.session.business.role = 'owner';
  assert.match(render(MembersPage), /option[^>]*value="owner"/);
  globalThis.__storeposTest.session.business.role = 'cashier';
  assert.match(render(MembersPage), /Akses dibatasi/);
  assert.doesNotMatch(render(MembersPage), /Undang anggota/);
});

test('satuan tambahan menampilkan konversi, faktor terkunci, dan harga server dengan tepat', () => {
  reset();
  globalThis.__storeposTest.resource = () => ({ data: { units: [{ unit_id: 'unit-a', name: 'Dus', factor_milli: '12000', sale_price_cents: '1234567', factor_locked: true }] }, loading: false, error: null, reload() {} });
  const html = render(ProductUnits, { product: { product_name: 'Produk uji', variant: { id: 'variant-a', sku: 'SKU-A', base_unit: 'pcs' } } });
  assert.match(html, /12 pcs/);
  assert.match(html, /Faktor terkunci/);
  assert.match(html, /value="12345.67"/);
  assert.match(html, /aria-label="Barcode satuan Dus"/);
  assert.match(html, /Tambah satuan/);
});

test('keypad memiliki tombol non-submit, label kontrol, dan dapat dikunci saat request', () => {
  reset();
  const html = render(NumericKeypad, { value: '125', onChange() {}, disabled: true });
  assert.equal((html.match(/type="button"/g) ?? []).length, 13);
  assert.equal((html.match(/disabled=""/g) ?? []).length, 13);
  assert.match(html, /aria-label="Hapus digit terakhir"/);
  assert.match(html, /aria-label="Kosongkan angka"/);
  assert.match(html, /role="group" aria-label="Keypad numerik"/);
});

test('riwayat transaksi memakai snapshot server dan ditolak untuk peran gudang', () => {
  reset();
  globalThis.__storeposTest.resource = () => ({ data: { sales: [{ sale_id: 'sale-a', receipt_number: 'UTAMA-0007', completed_at: '2026-10-13T03:00:00Z', total_cents: '2500000', status: 'completed', return_status: 'none', customer_name: null }] }, loading: false, error: null, reload() {} });
  const html = render(SalesPage);
  assert.match(html, /UTAMA-0007/);
  assert.match(html, /Rp 25.000/);
  assert.match(html, /aria-label="Nomor struk"/);
  assert.match(html, /Detail/);
  globalThis.__storeposTest.session.business.role = 'warehouse';
  assert.match(render(SalesPage), /Akses dibatasi/);
  assert.doesNotMatch(render(SalesPage), /UTAMA-0007/);
});

test('template struk menjaga nominal dan meng-escape nama produk', () => {
  reset();
  const receipt = { receipt_number: 'UTAMA-001', status: 'completed', completed_at: '2026-10-13T03:00:00Z', tax_cents: '50000', cash_rounding_cents: '100', total_cents: '2500100', change_cents: '499900', lines: [{ product_name: '<script>unsafe</script>', variant_name: '', unit: 'kg', qty_milli: '375', unit_price_cents: '1400000', line_total_cents: '525000' }] };
  const html = render(ReceiptPaper, { receipt, width: '58', copy: true });
  assert.match(html, /receipt-58/);
  assert.match(html, /SALINAN/);
  assert.match(html, /0,375 kg/);
  assert.match(html, /Rp 5.250/);
  assert.match(html, /Rp 25.001/);
  assert.match(html, /&lt;script&gt;/);
  assert.doesNotMatch(html, /<script>/);
  const wide = render(ReceiptPaper, { receipt });
  assert.match(wide, /receipt-80/);
  assert.doesNotMatch(wide, /SALINAN/);
});

test('layout memperingatkan jam melenceng hanya ketika melebihi lima menit', () => {
  reset();
  globalThis.__storeposTest.session.clockOffsetMs = 360000;
  assert.match(render(AdminLayout), /Jam perangkat tertinggal/);
  globalThis.__storeposTest.session.clockOffsetMs = -360000;
  assert.match(render(AdminLayout), /Jam perangkat mendahului/);
  globalThis.__storeposTest.session.clockOffsetMs = 300000;
  assert.doesNotMatch(render(AdminLayout), /Jam perangkat/);
});
test('generator varian menyediakan atribut, pratinjau kombinasi dan label formulir', () => {
  reset();
  const html = render(VariantBuilder, { categories: [] });
  assert.match(html, /Maksimal 3 atribut dan 100 kombinasi/);
  assert.match(html, /Buat pratinjau kombinasi/);
  assert.match(html, /Tambah atribut/);
  assert.match(html, /id="variant_product_name"/);
});

test('mode offline menampilkan katalog cache dan memblokir bayar/sinkronisasi tanpa koneksi', () => {
  reset();
  const html = render(OfflineCashier, { snapshot: { catalog: [{ product_name: 'Produk cache', variant: { id: 'variant-a', sku: 'CACHE-A', base_unit: 'pcs', price_cents: '100000' } }] }, pending: null, online: false, branch, register: { id: 'register-a', name: 'Kasir uji' } });
  assert.match(html, /Produk cache/);
  assert.match(html, /disabled=""[^>]*>Bayar/);
  assert.match(html, /disabled=""[^>]*>Sinkronkan draft ke server/);
  assert.match(html, /Harga cache bersifat sementara/);
});

test('keranjang cache menampilkan subtotal setelah diskon draft tersimpan', () => {
  reset();
  const snapshot = {
    catalog: [{ product_name: 'Es Teh Manis', variant: { id: 'tea', sku: 'RST-ESTEH', base_unit: 'gelas', price_cents: '800000' } }],
    active: { cart: { lines: [{ variant_id: 'tea', qty_milli: '1000' }], discount: { kind: 'amount', value: '100000' } } },
  };
  const html = render(OfflineCashier, { snapshot, pending: null, online: false, branch, register: { id: 'register-a', name: 'Kasir 1' } });
  assert.match(html, /Subtotal cache/);
  assert.match(html, /Rp 7\.000/);
  assert.doesNotMatch(html, /Rp 8\.000/);
});

test('struk restoran memakai modifier/catatan snapshot dan meng-escape teksnya', () => {
 const receipt={receipt_number:'R-1',status:'completed',completed_at:'2026-10-14T00:00:00Z',tax_cents:'0',cash_rounding_cents:'0',total_cents:'100000',change_cents:'0',lines:[{product_name:'Menu',qty_milli:'1000',unit:'porsi',unit_price_cents:'100000',line_total_cents:'100000',modifiers:[{option_id:'o',group_name:'Gula',name:'<b>Tanpa gula</b>'}],note:'<script>catatan</script>'}]};
 const html=render(ReceiptPaper,{receipt});
 assert.match(html,/Gula:/);
 assert.match(html,/&lt;b&gt;Tanpa gula&lt;\/b&gt;/);
 assert.match(html,/Catatan:/);
 assert.doesNotMatch(html,/<script>/);
});
