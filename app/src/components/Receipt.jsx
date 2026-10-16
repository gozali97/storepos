import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { formatIDR, formatQty } from '../money.js';
import { Notice } from './UI.jsx';
import { useSession } from '../auth/SessionContext.jsx';
import QRCode from 'qrcode';
import { printCopy } from '../receipts.js';

export function ReceiptPaper({ receipt, width = '80', copy = false }) {
  return <article className={`receipt-paper receipt-${width}`} aria-label="Struk transaksi">
    <header><h2>Struk transaksi</h2>{copy && <strong>SALINAN</strong>}<p>{receipt.receipt_number}</p><p>{new Date(receipt.completed_at).toLocaleString('id-ID')}</p><p>Status: {receipt.status}</p>{receipt.customer_name && <p>Pelanggan: {receipt.customer_name}</p>}</header>
    <div className="receipt-items">{receipt.lines.map((line, index) => <div key={index}><strong>{line.product_name}{line.variant_name ? ` · ${line.variant_name}` : ''}</strong>{line.modifiers?.map((modifier, i) => <small key={modifier.option_id ?? i}>{modifier.group_name ? `${modifier.group_name}: ` : ''}{modifier.name}</small>)}{line.note && <small>Catatan: {line.note}</small>}<div><span>{formatQty(BigInt(line.qty_milli))} {line.unit ?? line.sale_unit ?? line.base_unit} × {formatIDR(BigInt(line.unit_price_cents))}</span><b>{formatIDR(BigInt(line.line_total_cents))}</b></div></div>)}</div>
    <dl>{receipt.delivery_fee_cents && BigInt(receipt.delivery_fee_cents) > 0n && <div><dt>Ongkir</dt><dd>{formatIDR(BigInt(receipt.delivery_fee_cents))}</dd></div>}<div><dt>Pajak</dt><dd>{formatIDR(BigInt(receipt.tax_cents))}</dd></div><div><dt>Pembulatan</dt><dd>{formatIDR(BigInt(receipt.cash_rounding_cents))}</dd></div><div className="receipt-grand-total"><dt>Total</dt><dd>{formatIDR(BigInt(receipt.total_cents))}</dd></div><div><dt>Kembalian</dt><dd>{formatIDR(BigInt(receipt.change_cents))}</dd></div></dl>
  </article>;
}

export function ReceiptPreview({ receipt, onClose }) {
  const { client } = useSession();
  const dialog = useRef(null);
  const [width, setWidth] = useState('80');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  async function print() {
    if (busy) return;
    setBusy(true); setMessage('');
    try { await printCopy(client, receipt.sale_id, () => window.print()); }
    catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }
  useEffect(() => { dialog.current.showModal(); }, []);
  return createPortal(<dialog ref={dialog} className="receipt-preview" onClose={onClose} onCancel={event => { if (busy) event.preventDefault(); }}><div className="receipt-controls no-print"><h2>Pratinjau struk</h2><Notice>{message}</Notice><label>Lebar kertas<select value={width} onChange={event => setWidth(event.target.value)}><option value="58">58 mm</option><option value="80">80 mm</option></select></label><div className="row"><button className="primary" disabled={busy} onClick={print}>{busy ? 'Menyiapkan…' : 'Cetak'}</button><button disabled={busy} onClick={() => dialog.current.close()}>Tutup</button></div></div><ReceiptPaper receipt={receipt} width={width} copy /></dialog>, document.body);
}

export function ReceiptShare({ saleID }) {
  const { client, business } = useSession();
  const [url, setURL] = useState('');
  const [qr, setQR] = useState('');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  async function create() {
    setBusy(true); setMessage('');
    try {
      const result = await client.post(`/sales/${saleID}/receipt-links`, {});
      const link = new URL(result.receipt_path, window.location.origin).href;
      setURL(link);
      setQR(await QRCode.toDataURL(link, { width: 240, margin: 4, errorCorrectionLevel: 'M' }));
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }
  async function revoke() {
    setBusy(true); setMessage('');
    try {
      await client.del(`/sales/${saleID}/receipt-links`);
      setURL(''); setQR(''); setMessage('Seluruh tautan struk transaksi ini telah dicabut.');
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }
  return <section className="receipt-share"><h3>Struk digital</h3><Notice>{message}</Notice><div className="row"><button disabled={busy} onClick={create}>{url ? 'Buat tautan baru' : 'Buat tautan & QR'}</button>{business?.role === 'owner' && <button className="danger" disabled={busy} onClick={revoke}>Cabut semua tautan struk</button>}</div>{url && <div className="receipt-share-result">{qr && <img src={qr} width="240" height="240" alt="QR untuk membuka struk digital transaksi" />}<div><label>Tautan struk<input value={url} readOnly onFocus={event => event.target.select()} /></label><a className="text-link" href={url} target="_blank" rel="noopener noreferrer">Buka struk digital</a><p className="muted small">Bagikan tautan atau QR kepada pelanggan. Pemilik dapat mencabut aksesnya.</p></div></div>}</section>;
}
