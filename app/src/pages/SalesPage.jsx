import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { PageHeading, Notice, ResourceState } from '../components/UI.jsx';
import { formatIDR, formatQty } from '../money.js';
import { localDate } from '../reporting.js';
import { optionalApproval } from '../cashier.js';
import { returnLines, refundSummary } from '../returns.js';
import { newIdempotencyKey } from '../api/client.js';
import { ReceiptPreview, ReceiptShare } from '../components/Receipt.jsx';
import { SaleCorrectionHistory } from '../components/SaleCorrectionHistory.jsx';
import { PrescriptionReference } from '../components/PrescriptionReference.jsx';

export default function SalesPage() {
  const [params] = useSearchParams();
  const { businessId } = useSession();
  const { branch } = useWorkspace();
  const customerId = params.get('customer_id') ?? '';
  return <SalesWorkspace key={`${businessId}.${branch?.id}.${customerId}`} customerId={customerId} />;
}

function SalesWorkspace({ customerId }) {
  const { client, business, businessId, user } = useSession();
  const { branch, register } = useWorkspace();
  const allowed = ['owner', 'manager', 'cashier', 'pharmacist'].includes(business?.role);
  const [from, setFrom] = useState(localDate(branch?.timezone));
  const [to, setTo] = useState(localDate(branch?.timezone));
  const [receipt, setReceipt] = useState('');
  const [search, setSearch] = useState('');
  const [cursor, setCursor] = useState('');
  const [selected, setSelected] = useState('');
  const [action, setAction] = useState('');
  const [entries, setEntries] = useState({});
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');
  const [printPreview, setPrintPreview] = useState(false);
  const storageKey = `storepos.saleCorrection.${businessId}.${user?.id}`;
  const [pending, setPending] = useState(() => {
    try { return JSON.parse(sessionStorage.getItem(storageKey)) || null; } catch { return null; }
  });
  const query = new URLSearchParams({ branch_id: branch?.id ?? '', ...(customerId ? { customer_id: customerId } : {}), ...(search ? { receipt_number: search } : { from, to }), ...(cursor ? { before: cursor } : {}) });
  const resource = useResource(allowed && branch && (search || from && to && from <= to) ? `/sales?${query}` : null);
  const detail = useResource(allowed && selected ? `/sales/${selected}` : null);
  const data = detail.data;
  const approvalRequired = action === 'void' || ['cashier', 'pharmacist'].includes(business?.role);

  function choose(id) { setSelected(id); setAction(''); setEntries({}); setMessage(''); }
  function updateEntry(id, patch) { setEntries(current => ({ ...current, [id]: { qty: '', condition: 'restock', ...current[id], ...patch } })); }
  function createCorrection(event) {
    event.preventDefault();
    if (busy || pending) return;
    const form = new FormData(event.currentTarget);
    try {
      const approval = optionalApproval(form.get('approval_email'), form.get('approval_pin'));
      if (approvalRequired && !approval) throw new Error('Persetujuan atasan diperlukan.');
      const body = { reason: String(form.get('reason')).trim(), ...(approval ? { approval } : {}) };
      if (!body.reason) throw new Error('Alasan wajib diisi.');
      if (action === 'returns') {
        body.lines = returnLines(data.lines, entries);
        if (register) body.register_id = register.id;
        if (data.payments.some(payment => payment.method === 'cash') && (!register || branch?.id !== data.sale.branch_id)) throw new Error('Pilih terminal pada cabang transaksi dengan shift terbuka untuk refund tunai.');
      }
      const operation = { path: `/sales/${selected}/${action}`, body, key: newIdempotencyKey() };
      sessionStorage.setItem(storageKey, JSON.stringify(operation));
      setPending(operation); sendCorrection(operation);
    } catch (error) { setMessage(error.message); }
  }
  async function sendCorrection(operation) {
    if (busy) return;
    setBusy(true); setMessage(''); setSuccess('');
    try {
      const result = await client.post(operation.path, operation.body, { idempotencyKey: operation.key });
      sessionStorage.removeItem(storageKey); setPending(null); setAction(''); setEntries({});
      setSuccess(result.amount_cents ? `Retur disimpan. Refund ${formatIDR(BigInt(result.amount_cents))}: ${refundSummary(result.refunds).map(part => `${part.label} ${formatIDR(BigInt(part.cents))}${part.manual ? ' (kembalikan manual)' : ' (kas shift)'}`).join(', ')}.` : 'Transaksi berhasil dibatalkan. Periksa pembayaran nontunai yang perlu dikembalikan manual.');
      resource.reload(); detail.reload();
    } catch (error) {
      if (error.status && error.status < 500) { sessionStorage.removeItem(storageKey); setPending(null); setMessage(error.message); }
      else setMessage('Status koreksi belum pasti. Periksa & coba lagi dengan kunci yang sama.');
    } finally { setBusy(false); }
  }

  if (!allowed) return <section className="panel empty-state"><h1>Akses dibatasi</h1><p>Riwayat transaksi tidak tersedia untuk peran Anda.</p></section>;
  return <>
    <PageHeading title="Riwayat transaksi" description="Cari transaksi, periksa pembayaran, dan catat koreksi berdasarkan penjualan asli." />
    {customerId && <p className="muted" role="status">Filter pelanggan aktif. Riwayat ditampilkan untuk cabang dan periode yang dipilih.</p>}
  <Notice>{message}</Notice><Notice kind="success">{success}</Notice>
    {pending && <div className="alert warn" role="status"><p>Koreksi masih menunggu kepastian. Formulir dikunci agar tidak tercatat dua kali.</p><button disabled={busy} onClick={() => sendCorrection(pending)}>Periksa & coba lagi</button></div>}
    <section className="panel form-panel"><div className="table-toolbar"><div className="filter-fields"><label>Dari<input type="date" value={from} max={to} onChange={event => { setFrom(event.target.value); setCursor(''); }} /></label><label>Sampai<input type="date" value={to} min={from} onChange={event => { setTo(event.target.value); setCursor(''); }} /></label></div><form className="search-form" onSubmit={event => { event.preventDefault(); setSearch(receipt.trim()); setCursor(''); }}><input aria-label="Nomor struk" value={receipt} maxLength={100} placeholder="Nomor struk persis" onChange={event => setReceipt(event.target.value)} /><button>Cari</button></form></div>
      {search && <p className="report-note">Mencari nomor {search} di seluruh tanggal. Kosongkan pencarian untuk memakai periode.</p>}
      {!branch ? <p>Pilih cabang terlebih dahulu.</p> : !search && from > to ? <Notice>Periode tanggal tidak valid.</Notice> : <ResourceState resource={resource} empty={!resource.data?.sales?.length} message="Tidak ada transaksi yang sesuai dengan filter."><div className="table-scroll"><table><thead><tr><th>Struk</th><th>Waktu cabang</th><th>Pelanggan</th><th>Status</th><th className="num">Total</th><th>Aksi</th></tr></thead><tbody>{resource.data?.sales?.map(sale => <tr key={sale.sale_id}><td>{sale.receipt_number}</td><td>{new Date(sale.completed_at).toLocaleString('id-ID', { timeZone: branch.timezone })}</td><td>{sale.customer_name ?? '—'}</td><td>{sale.status} · {sale.return_status}</td><td className="num">{formatIDR(BigInt(sale.total_cents))}</td><td><button disabled={busy || !!pending} onClick={() => choose(sale.sale_id)}>Detail</button></td></tr>)}</tbody></table></div></ResourceState>}
      <div className="table-footer">{cursor && <button onClick={() => setCursor('')}>Halaman pertama</button>}<button disabled={!resource.data?.next_cursor || busy} onClick={() => setCursor(resource.data.next_cursor)}>Lebih lama</button></div>
    </section>
    {selected && <section className="panel form-panel"><ResourceState resource={detail}>{data && <>
      <div className="panel-heading"><div><h2>{data.sale.receipt_number}</h2><p className="muted">{data.sale.status} · Total {formatIDR(BigInt(data.sale.total_cents))} · Kembalian {formatIDR(BigInt(data.sale.change_cents))}</p></div><button disabled={busy || !!pending} onClick={() => setSelected('')}>Tutup</button></div>
      <div className="table-scroll"><table><thead><tr><th>Produk/SKU</th><th>Satuan</th><th className="num">Terjual</th><th className="num">Sisa retur</th><th className="num">Harga satuan</th><th className="num">Total baris</th></tr></thead><tbody>{data.lines.map(line => <tr key={line.sale_line_id}><td>{line.product_name} {line.variant_name}<small className="table-subtext">{line.sku}</small></td><td>{line.sale_unit ?? line.base_unit}</td><td className="num">{formatQty(BigInt(line.qty_milli))}</td><td className="num">{formatQty(BigInt(line.remaining_qty_milli))}</td><td className="num">{formatIDR(BigInt(line.unit_price_cents))}</td><td className="num">{formatIDR(BigInt(line.line_total_cents))}</td></tr>)}</tbody></table></div>
      <div className="payment-list">{data.payments.map((payment, index) => <div key={index}><strong>{payment.method} · {payment.status}</strong><b>{formatIDR(BigInt(payment.amount_cents))}</b>{payment.manual_refund_required && <span>Refund manual diperlukan</span>}</div>)}</div>
      <div className="row"><button onClick={() => setPrintPreview(true)}>Pratinjau cetak 58 / 80 mm</button></div>
      {printPreview && <ReceiptPreview receipt={{ ...data.sale, lines: data.lines }} onClose={() => setPrintPreview(false)} />}
      <ReceiptShare key={selected} saleID={selected} />
      <PrescriptionReference saleId={selected} />
      <SaleCorrectionHistory key={`${selected}-${data.sale.status}-${data.sale.return_status}`} saleId={selected} timezone={branch?.timezone} />
      {data.sale.status === 'completed' && <div className="row"><button disabled={busy || !!pending} onClick={() => setAction('void')}>Void transaksi</button><button disabled={busy || !!pending || data.sale.return_status === 'full'} onClick={() => setAction('returns')}>Retur barang</button></div>}
      {action && <form className="stock-form" onSubmit={createCorrection}><fieldset disabled={busy || !!pending}><h3>{action === 'void' ? 'Void transaksi' : 'Retur barang'}</h3><p className="report-note">Void memerlukan shift asal terbuka. Refund dibagi ke metode pembayaran asal: nontunai dulu (dikembalikan manual), sisanya tunai dari shift terminal aktif.</p>
        {action === 'returns' && <div className="return-lines">{data.lines.filter(line => BigInt(line.remaining_qty_milli) > 0n).map(line => <div key={line.sale_line_id}><label><input type="checkbox" checked={!!entries[line.sale_line_id]?.selected} onChange={event => updateEntry(line.sale_line_id, { selected: event.target.checked })} />{line.product_name} · {line.sale_unit ?? line.base_unit}</label><div className="form-grid"><label className="field">Jumlah<input type="number" min={line.allow_fractional ? '0.001' : '1'} step={line.allow_fractional ? '0.001' : '1'} max={formatQty(BigInt(line.remaining_qty_milli)).replace(',', '.')} required={!!entries[line.sale_line_id]?.selected} value={entries[line.sale_line_id]?.qty ?? ''} onChange={event => updateEntry(line.sale_line_id, { qty: event.target.value })} /></label><label className="field">Kondisi<select value={entries[line.sale_line_id]?.condition ?? 'restock'} onChange={event => updateEntry(line.sale_line_id, { condition: event.target.value })}><option value="restock">{business?.business_type === 'pharmacy' ? 'Retur — masuk karantina batch asal' : 'Layak jual — kembali ke stok'}</option><option value="damaged">Rusak — tidak menambah stok jual</option></select></label></div></div>)}</div>}
        <div className="form-grid"><label className="field">Alasan<input name="reason" required maxLength={200} /></label><label className="field">Email atasan<input name="approval_email" type="email" required={approvalRequired} autoComplete="username" /></label><label className="field">PIN atasan<input name="approval_pin" type="password" inputMode="numeric" pattern="[0-9]{6}" maxLength={6} required={approvalRequired} autoComplete="off" /></label></div><div className="row"><button className="primary">{action === 'void' ? 'Konfirmasi void' : 'Simpan retur'}</button><button type="button" onClick={() => setAction('')}>Batal</button></div>
      </fieldset></form>}
    </>}</ResourceState></section>}
  </>;
}
