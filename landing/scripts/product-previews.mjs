import { createRequire } from 'node:module';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

const appRoot = fileURLToPath(new URL('../../app/', import.meta.url));
const require = createRequire(new URL('../../app/package.json', import.meta.url));
const { build } = require('esbuild');
let cached;

export async function productPreviews() {
  if (cached) return cached;
  const result = await build({
    stdin: {
      contents: `export { default as HomePage } from './src/pages/HomePage.jsx'; export { default as ReportsPage } from './src/pages/ReportsPage.jsx'; export { ReceiptPaper } from './src/components/Receipt.jsx';`,
      resolveDir: appRoot, loader: 'jsx'
    },
    bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic',
    external: ['react', 'react-dom', 'react-router-dom'],
    plugins: [{ name: 'demo-data', setup(builder) {
      builder.onResolve({ filter: /(?:SessionContext|WorkspaceContext)\.jsx$|useResource\.js$/ }, () => ({ path: 'demo', namespace: 'demo' }));
      builder.onLoad({ filter: /.*/, namespace: 'demo' }, () => ({ contents: `
        export function useSession() { return { user: { name: 'Tim Sumber Rejeki' }, business: { business_name: 'Toko Sumber Rejeki', business_type: 'retail', role: 'owner' }, client: {} }; }
        export function useWorkspace() { return { branch: { id: 'demo', name: 'Cabang Utama', timezone: 'Asia/Jakarta' }, register: { id: 'demo', name: 'Kasir 1' } }; }
        export function useResource(path = '') {
          const products = ['Beras 5 kg', 'Minyak Goreng 2 L', 'Gula Pasir 1 kg', 'Telur'];
          const today = new Intl.DateTimeFormat('en-CA', {timeZone: 'Asia/Jakarta', year: 'numeric', month: '2-digit', day: '2-digit'}).format(new Date());
          let rows = [];
          if (path?.includes('/reports/sales')) rows = Array.from({ length: 7 }, (_, i) => {
            const date = new Date(today + 'T12:00:00Z'); date.setUTCDate(date.getUTCDate() - 6 + i);
            const gross = [32000000, 45000000, 38000000, 60000000, 52000000, 75000000, 100000000][i];
            return { date: date.toISOString().slice(0,10), sale_count: [8,12,10,15,13,18,24][i], gross_cents: String(gross), net_cents: String(gross), discount_cents: '0', returns_cents: '0', tax_cents: '0' };
          });
          if (path?.includes('/reports/products')) rows = products.map((product_name, i) => ({ product_name, sold_base_qty_milli: String([12000,20000,24000,6500][i]), net_sales_cents: String([90000000,70000000,36000000,19500000][i]) }));
          if (path?.includes('/reports/payments')) rows = [{method:'Tunai', verification:'MANUAL', net_cents:'100000000'}, {method:'QRIS',verification:'MANUAL',net_cents:'18000000'}];
          if (path?.includes('/reports/stock')) rows = products.map((product_name, i) => ({ product_name, sku: 'SR-00' + (i+1), base_unit: i === 3 ? 'kg' : 'pcs', qty_milli: String([35000,24000,40000,12500][i]), minimum_qty_milli: '10000' }));
          return { data: { rows, categories: [] }, loading: false, error: null, reload() {} };
        }
      ` }));
    } }]
  });
  const module = { exports: {} };
  new Function('require', 'module', 'exports', result.outputFiles[0].text)(require, module, module.exports);
  const { createElement } = require('react');
  const { renderToStaticMarkup } = require('react-dom/server');
  const { StaticRouter } = require('react-router-dom');
  const { HomePage, ReportsPage, ReceiptPaper } = module.exports;
  const css = (await Promise.all(['tokens.css', 'app.css'].map(file => readFile(appRoot + 'src/styles/' + file, 'utf8')))).join('\n');
  const receipt = { receipt_number: 'SR-DEMO-001', completed_at: '2026-10-14T03:00:00Z', status: 'completed', tax_cents: '0', cash_rounding_cents: '0', total_cents: '17500000', change_cents: '2500000', lines: [
    { product_name: 'Beras 5 kg', qty_milli: '1000', unit: 'pcs', unit_price_cents: '7500000', line_total_cents: '7500000' },
    { product_name: 'Minyak Goreng 2 L', qty_milli: '1000', unit: 'pcs', unit_price_cents: '4500000', line_total_cents: '4500000' },
    { product_name: 'Gula Pasir 1 kg', qty_milli: '2000', unit: 'pcs', unit_price_cents: '2000000', line_total_cents: '4000000' },
    { product_name: 'Telur', qty_milli: '500', unit: 'kg', unit_price_cents: '3000000', line_total_cents: '1500000' }
  ] };
  cached = {};
  for (const [name, component, props] of [['dashboard', HomePage, {}], ['reports', ReportsPage, {}], ['stock', ReportsPage, { stockOnly: true }], ['receipt', ReceiptPaper, { receipt }]]) {
    const content = renderToStaticMarkup(createElement(StaticRouter, { location: '/' }, createElement(component, props)));
    cached[name] = `<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="robots" content="noindex"><title>StorePOS · ${name} · Data demo</title><style>${css}\nbody{padding:24px;background:#f8f9fc} .demo-label{display:block;margin-bottom:16px;color:#1d4ed8;font-size:12px;font-weight:700} .receipt-paper{width:min(100%,320px)}</style></head><body><span class="demo-label">DATA DEMO · Toko Sumber Rejeki</span><main inert>${content}</main></body></html>`;
  }
  return cached;
}
