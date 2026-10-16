import { useEffect, useRef, useState } from 'react';
import { formatIDR } from '../money.js';
import { Notice } from '../components/UI.jsx';

const presets = {
  a4_3x8: { label: 'A4 · 3 kolom × 8 baris', page: 'A4 portrait', columns: 3, width: '63.3mm', height: '33.1mm', margin: '8mm', gap: '2mm' },
  a4_2x7: { label: 'A4 · 2 kolom × 7 baris', page: 'A4 portrait', columns: 2, width: '99mm', height: '39mm', margin: '5mm', gap: '2mm' },
  thermal: { label: 'Thermal · 50 × 30 mm', page: '50mm 30mm', columns: 1, width: '50mm', height: '30mm', margin: '0', gap: '0' }
};

function Barcode({ value, encoder }) {
  const ref = useRef(null);
  const [error, setError] = useState('');
  useEffect(() => {
    if (!encoder) return;
    try {
      encoder(ref.current, value, { format: 'CODE128', displayValue: false, margin: 0, width: 1.4, height: 40 });
      setError('');
    } catch {
      setError('Kode ini tidak bisa dibuat sebagai Code 128.');
    }
  }, [value]);
  return <><svg ref={ref} className="barcode-symbol" role="img" aria-label={`Barcode ${value}`} />{error && <span className="barcode-error" role="alert">{error}</span>}</>;
}

function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]);
}

function barcodeSVG(value, encoder) {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  encoder(svg, value, { format: 'CODE128', displayValue: false, margin: 0, width: 1.4, height: 40 });
  return svg.outerHTML;
}

export default function BarcodeLabels({ items, onClose }) {
  const [preset, setPreset] = useState('a4_3x8');
  const [selected, setSelected] = useState({});
  const [message, setMessage] = useState('');
  const [encoder, setEncoder] = useState(null);
  useEffect(() => {
    let active = true;
    import('jsbarcode').then(module => {
      if (active) setEncoder(() => module.default);
    }).catch(() => {
      if (active) setMessage('Generator barcode gagal dimuat. Muat ulang halaman untuk mencoba lagi.');
    });
    return () => { active = false; };
  }, []);
  const selectedRows = items.flatMap(item => {
    const codes = item.variant.barcodes?.length ? item.variant.barcodes : [item.variant.sku];
    return codes.map((code, index) => {
      const key = `${item.variant.id}:${index}:${code}`;
      const row = selected[key];
      return row ? { key, code, qty: Number(row.qty), item } : null;
    }).filter(Boolean);
  });
  const labels = selectedRows.flatMap(row => Array.from({ length: row.qty }, (_, index) => ({ ...row, copy: index + 1 })));

  function updateSelection(key, checked) {
    setSelected(current => {
      const next = { ...current };
      if (checked) next[key] = { checked: true, qty: 1 };
      else delete next[key];
      return next;
    });
  }

  function updateQty(key, value) {
    const qty = Number(value);
    setSelected(current => ({ ...current, [key]: { ...current[key], qty: Number.isInteger(qty) ? Math.min(200, Math.max(1, qty)) : 1 } }));
  }

  function printLabels() {
    setMessage('');
    if (!labels.length) {
      setMessage('Pilih barcode dan jumlah label terlebih dahulu.');
      return;
    }
    let symbols;
    try {
      symbols = labels.map(label => barcodeSVG(label.code, encoder));
    } catch {
      setMessage('Ada barcode yang tidak dapat dibuat sebagai Code 128. Periksa isi barcode pada katalog.');
      return;
    }
    const popup = window.open('', '_blank');
    if (!popup) {
      setMessage('Jendela cetak diblokir browser. Izinkan pop-up lalu coba lagi.');
      return;
    }
    const layout = presets[preset];
    const content = labels.map((label, index) => {
      const name = index === 0 || labels[index - 1].item.variant.id !== label.item.variant.id ? `<strong>${escapeHTML(label.item.product_name)}</strong>` : '';
      return `<article class="label">${name}<small>${escapeHTML(label.item.variant.name || label.item.variant.sku)}</small>${symbols[index]}<span class="code">${escapeHTML(label.code)}</span></article>`;
    }).join('');
    popup.document.write(`<!doctype html><html><head><meta charset="utf-8"><title>Label barcode</title><style>@page{size:${layout.page};margin:${layout.margin}}*{box-sizing:border-box}html,body{margin:0;color:#111;background:#fff;font:9pt Arial,sans-serif}.labels{display:grid;grid-template-columns:repeat(${layout.columns},${layout.width});grid-auto-rows:${layout.height};gap:${layout.gap}}.label{width:${layout.width};height:${layout.height};padding:2mm;display:flex;flex-direction:column;justify-content:center;align-items:center;overflow:hidden;text-align:center;break-inside:avoid;${preset === 'thermal' ? 'break-after:page;' : ''}}.label strong,.label small,.label .code{max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.label strong{font-size:9pt}.label small{font-size:7pt;margin-top:1mm}.label svg{display:block;width:100%;max-height:15mm}.label .code{font:7pt monospace;margin-top:1mm}@media print{.label{break-inside:avoid}}</style></head><body><main class="labels">${content}</main></body></html>`);
    popup.document.close();
    popup.focus();
    window.setTimeout(() => popup.print(), 200);
  }

  return <section className="panel barcode-labels no-print" aria-labelledby="barcode-label-heading">
    <div className="panel-heading"><div><h2 id="barcode-label-heading">Cetak label barcode</h2><p className="muted small">Pilih barcode yang sudah tersimpan. Jika produk belum memiliki barcode, SKU dipakai sebagai kode label.</p></div><button type="button" onClick={onClose}>Tutup</button></div>
    <Notice>{message}</Notice>
    <div className="filter-fields barcode-label-controls">
      <label>Format label<select value={preset} onChange={event => setPreset(event.target.value)}>{Object.entries(presets).map(([key, value]) => <option key={key} value={key}>{value.label}</option>)}</select></label>
      <p className="muted small">Jumlah label: {labels.length} · maksimum 200 per kode</p>
      <button type="button" className="primary" disabled={!labels.length || !encoder} onClick={printLabels}>{encoder ? `Cetak ${labels.length || ''} label` : 'Memuat generator…'}</button>
    </div>
    <div className="barcode-label-list">
      {items.flatMap(item => (item.variant.barcodes?.length ? item.variant.barcodes : [item.variant.sku]).map((code, index) => {
        const key = `${item.variant.id}:${index}:${code}`;
        const checked = Boolean(selected[key]);
        return <div className="barcode-label-option" key={key}>
          <label><input type="checkbox" checked={checked} onChange={event => updateSelection(key, event.target.checked)} /><span><strong>{item.product_name}</strong><small>{item.variant.name || item.variant.sku} · {item.variant.base_unit} · {code}</small></span></label>
          <span className="muted small">{formatIDR(BigInt(item.variant.price_cents))}</span>
          {checked && <label className="barcode-qty">Jumlah<input type="number" min="1" max="200" value={selected[key].qty} onChange={event => updateQty(key, event.target.value)} /></label>}
        </div>;
      }))}
      {!items.length && <p className="empty-state">Tidak ada produk pada hasil pencarian saat ini.</p>}
    </div>
    {labels.length > 0 && <div className="barcode-label-preview" aria-label="Pratinjau label"><h3>Pratinjau</h3><div className={`barcode-preview-grid preset-${preset}`}>{labels.slice(0, 6).map((label, index) => <div className="barcode-label-card" key={`${label.key}-${index}`}><strong>{label.item.product_name}</strong><small>{label.item.variant.name || label.item.variant.sku}</small>{encoder ? <Barcode value={label.code} encoder={encoder} /> : <span role="status">Memuat barcode…</span>}<span>{label.code}</span></div>)}</div>{labels.length > 6 && <p className="muted small">Menampilkan 6 dari {labels.length} label.</p>}</div>}
  </section>;
}
