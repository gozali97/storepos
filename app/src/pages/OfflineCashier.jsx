import { useMemo, useState } from 'react';
import { addProduct, cartSubtotal, normalizeCart, quantityToMilli } from '../cashier.js';
import { offlineCart, syncOfflineCart } from '../offline.js';
import { formatIDR, formatQty } from '../money.js';
import { Notice } from '../components/UI.jsx';

export default function OfflineCashier({ snapshot, pending, storageKey, online, client, branch, register, onPending, onSynced, onClose }) {
  const [data, setData] = useState(() => pending ?? offlineCart(snapshot));
  const [query, setQuery] = useState('');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const products = useMemo(() => new Map(data.catalog.map(item => [item.variant.id, item])), [data.catalog]);
  const units = useMemo(() => new Map(Object.values(data.productUnits).flat().map(unit => [unit.unit_id, unit])), [data.productUnits]);
  const subtotal = cartSubtotal(data.cart, products, units);
  const results = data.catalog.filter(item => `${item.product_name} ${item.variant.name ?? ''} ${item.variant.sku} ${(item.variant.barcodes ?? []).join(' ')}`.toLowerCase().includes(query.trim().toLowerCase()));

  function save(cart) {
    try {
      const next = { ...data, cart };
      localStorage.setItem(storageKey, JSON.stringify(next));
      setData(next); onPending(next); setMessage('');
    } catch { setMessage('Keranjang tidak dapat disimpan pada perangkat. Perubahan belum diterapkan; periksa ruang penyimpanan browser.'); }
  }
  function changeQty(index, value) {
    try {
      const next = normalizeCart(data.cart);
      next.lines[index].qty_milli = quantityToMilli(value);
      save(next);
    } catch (error) { setMessage(error.message); }
  }
  function scan(event) {
    event.preventDefault();
    const value = query.trim();
    const exact = data.catalog.find(item => item.variant.sku.toLowerCase() === value.toLowerCase());
    if (exact) { save(addProduct(data.cart, exact)); setQuery(''); }
    else setMessage(results.length ? 'Pilih produk dan satuan dari hasil cache. Barcode satuan diperiksa kembali setelah online.' : 'Produk belum tersedia di cache. Muat katalog saat koneksi kembali.');
  }
  async function sync() {
    if (!online || busy) return;
    setBusy(true); setMessage('');
    try {
      const saved = await syncOfflineCart(client, data, branch.id, register.id);
      localStorage.removeItem(storageKey);
      onSynced(saved);
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }
  function discard() {
    if (!window.confirm('Hapus perubahan keranjang offline dari perangkat? Draft server tetap tersimpan.')) return;
    localStorage.removeItem(storageKey); onPending(null); onClose();
  }
  return <section className="panel form-panel"><div className="panel-heading"><div><h2>Keranjang offline</h2><p className="muted">{branch.name} · {register.name}. Harga cache bersifat sementara; pembayaran tersedia setelah sinkronisasi.</p></div>{!pending && online && <button onClick={onClose}>Kembali</button>}</div><Notice>{message}</Notice>
    <form className="search-form" onSubmit={scan}><input aria-label="Cari atau scan produk cache" autoFocus value={query} onChange={event => setQuery(event.target.value)} placeholder="Nama, SKU, barcode" /><button disabled={busy}>Cari / scan</button></form>
    <div className="offline-products">{results.slice(0, 60).map(item => <div key={item.variant.id}><button disabled={busy} onClick={() => save(addProduct(data.cart, item))}>{item.product_name} · {item.variant.name || item.variant.sku} · {item.variant.base_unit}</button>{(data.productUnits[item.variant.id] ?? []).filter(unit => unit.sale_price_cents != null).map(unit => <button key={unit.unit_id} disabled={busy} onClick={() => save(addProduct(data.cart, item, unit))}>{unit.name} · {formatIDR(BigInt(unit.sale_price_cents))}</button>)}</div>)}</div>
    {!data.catalog.length && <p className="report-note">Cache katalog masih kosong. Buka kasir saat online untuk memuat produk.</p>}
    <div className="table-scroll"><table><thead><tr><th>Produk</th><th>Satuan</th><th>Jumlah</th><th>Aksi</th></tr></thead><tbody>{data.cart.lines.map((line, index) => <tr key={`${line.variant_id}-${line.unit_id}-${line.qty_milli}`}><td>{products.get(line.variant_id)?.product_name ?? line.variant_id}</td><td>{units.get(line.unit_id)?.name ?? products.get(line.variant_id)?.variant.base_unit ?? '—'}</td><td><input aria-label={`Jumlah offline ${index + 1}`} type="number" min="0.001" step="0.001" defaultValue={formatQty(BigInt(line.qty_milli)).replace(',', '.')} disabled={busy} onBlur={event => changeQty(index, event.target.value)} /></td><td><button className="danger" disabled={busy} onClick={() => { const next = normalizeCart(data.cart); next.lines.splice(index, 1); save(next); }}>Hapus</button></td></tr>)}</tbody></table></div>
    <div className="cart-total"><span>Subtotal cache</span><strong>{subtotal == null ? '—' : formatIDR(subtotal)}</strong><small>Total final mengikuti harga dan aturan server setelah sinkronisasi.</small></div>
    <div className="row"><button className="primary" disabled={!online || busy || !pending} onClick={sync}>{busy ? 'Menyinkronkan…' : 'Sinkronkan draft ke server'}</button><button disabled>Bayar (dibutuhkan koneksi)</button>{pending && <button className="danger" disabled={busy} onClick={discard}>Hapus keranjang offline</button>}</div>
  </section>;
}
