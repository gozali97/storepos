import { useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { usePendingMutation } from '../hooks/usePendingMutation.js';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { newIdempotencyKey } from '../api/client.js';
import { rupiahToCents } from '../forms.js';
import { formatIDR, formatQty } from '../money.js';
import { localDate } from '../reporting.js';
import { batchActionBody, batchCountBody, batchReportPath, expiryLabel, receiptBody } from '../pharmacy.js';
import { PageHeading, Field, Notice, ResourceState } from '../components/UI.jsx';

const FILTERS = [['all', 'Semua'], ['near_expiry', 'Mendekati ED'], ['expired', 'Kedaluwarsa'], ['quarantine', 'Karantina']];

export default function PharmacyPage() {
  const { businessId, user } = useSession();
  const { branch } = useWorkspace();
  return <PharmacyWorkspace key={`${businessId}.${user?.id}.${branch?.id}`} />;
}

function PharmacyWorkspace() {
  const { client, business, businessId, user } = useSession();
  const { branch } = useWorkspace();
  const mutation = usePendingMutation(`storepos.pendingPharmacy.${businessId}.${user?.id}.${branch?.id}`);
  const [filter, setFilter] = useState('near_expiry');
  const [show, setShow] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');
  const [operation, setOperation] = useState(null); // {kind,item}
  const [trace, setTrace] = useState(null);
  const [from, setFrom] = useState(localDate(branch?.timezone));
  const [to, setTo] = useState(localDate(branch?.timezone));
  const form = useRef(null);
  const canManage = ['owner', 'manager', 'warehouse', 'pharmacist'].includes(business?.role);
  const canRelease = business?.role === 'pharmacist';
  const batches = useResource(branch ? `/pharmacy/batches?branch_id=${branch.id}&filter=${filter}` : null);
  const catalog = useResource(show ? '/catalog/search?limit=100' : null);
  const suppliers = useResource(show ? '/suppliers' : null);
  const items = batches.data?.batches ?? [];
  const warn = batches.data?.expiry_warning_days ?? 90;
  const products = (catalog.data?.items ?? []).filter(item => item.track_stock);

  async function sendOperation(op) {
    setBusy(true); setMessage(''); setSuccess('');
    try {
      await mutation.send(op);
      setSuccess('Aksi batch berhasil dicatat.');
      setOperation(null);
      if (op.path === '/pharmacy/batch-receipts') { form.current?.reset(); setShow(false); }
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); batches.reload(); }
  }

  function receive(event) {
    event.preventDefault();
    const v = Object.fromEntries(new FormData(event.currentTarget));
    if (busy || mutation.blocked) return;
    try {
      const body = receiptBody(branch.id, { ...v, unit_cost_cents: rupiahToCents(v.unit_cost) });
      sendOperation({ path: '/pharmacy/batch-receipts', body, key: newIdempotencyKey() });
    } catch (error) { setMessage(error.message); }
  }

  function submitOperation(event) {
    event.preventDefault();
    const values = Object.fromEntries(new FormData(event.currentTarget));
    if (busy || mutation.blocked) return;
    const { kind, item } = operation;
    try {
      const body = kind === 'count'
        ? batchCountBody(branch.id, item.stored_status, values.counted, values.reason)
        : batchActionBody(branch.id, kind, item.stored_status, values.qty, values.reason);
      sendOperation({ path: `/pharmacy/batches/${item.batch_id}/${kind}`, body, key: newIdempotencyKey() });
    } catch (error) { setMessage(error.message); }
  }

  async function loadTrace(item) {
    setBusy(true); setMessage('');
    try {
      const data = await client.get(`/pharmacy/batches/${item.batch_id}/trace?branch_id=${branch.id}`);
      setTrace({ item, ...data });
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }

  function saveBlob(blob, filename) {
    const url = URL.createObjectURL(blob); const a = document.createElement('a');
    a.href = url; a.download = filename; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  async function downloadReport() {
    if (!branch || from > to) return;
    setBusy(true); setMessage('');
    try { saveBlob(await client.get(batchReportPath(branch.id, from, to, filter, warn, 'csv'), { responseType: 'blob' }), `batch-stock-${from}-${to}.csv`); }
    catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }

  return <>
    <PageHeading title="Batch & kedaluwarsa" description="Pantau batch, ED, karantina, opname, dan riwayatnya.">
      <Link to="/apotek/kelola">Laporan, kartu batch & identitas obat</Link>
      {canManage && <button className="primary" disabled={!branch} onClick={() => setShow(v => !v)}>{show ? 'Tutup formulir' : 'Terima batch'}</button>}
    </PageHeading>
    <Notice>{message}</Notice><Notice kind="success">{success}</Notice><Notice>{mutation.error}</Notice>
    {mutation.pending && <div className="alert warn" role="status">Status aksi batch belum pasti. Jangan membuat aksi baru.<button disabled={busy || mutation.sending} onClick={() => sendOperation(mutation.pending)}>Periksa & coba lagi dengan data yang sama</button></div>}
    {show && <section className="panel form-panel"><h2>Penerimaan batch</h2><form ref={form} onSubmit={receive}><fieldset disabled={busy || mutation.blocked}><div className="form-grid">
      <Field label="Obat" id="variant_id"><select id="variant_id" name="variant_id" required><option value="">Pilih obat</option>{products.map(i => <option key={i.variant.id} value={i.variant.id}>{i.product_name} · {i.variant.sku}</option>)}</select></Field>
      <Field label="Supplier" id="supplier_id"><select id="supplier_id" name="supplier_id" required><option value="">Pilih supplier</option>{(suppliers.data?.suppliers ?? []).filter(s => !s.archived).map(s => <option key={s.id} value={s.id}>{s.name}</option>)}</select></Field>
      <Field label="Nomor batch" id="batch_number" required maxLength={100} /><Field label="ED (YYYY-MM-DD atau YYYY-MM)" id="expiry_date" required placeholder="2027-03" />
      <Field label="Nomor faktur" id="invoice_number" maxLength={200} /><Field label="Jumlah (satuan dasar)" id="qty" type="number" min="1" step="1" required />
      <Field label="Harga beli per satuan (Rp)" id="unit_cost" type="number" step="0.01" min="0" required /><Field label="Status" id="status"><select id="status" name="status"><option value="SELLABLE">Dapat dijual</option><option value="QUARANTINE">Karantina</option></select></Field>
    </div><button className="primary">Simpan penerimaan</button></fieldset></form></section>}

    <section className="panel"><div className="table-toolbar"><h2>Stok per batch</h2><div className="row">{FILTERS.map(([v, label]) => <button key={v} aria-pressed={filter === v} className={filter === v ? 'primary' : ''} onClick={() => setFilter(v)}>{label}</button>)}</div></div>
      <div className="filter-fields"><label>Dari<input type="date" value={from} max={to} onChange={e => setFrom(e.target.value)} /></label><label>Sampai<input type="date" value={to} min={from} onChange={e => setTo(e.target.value)} /></label><button disabled={busy || !branch || from > to} onClick={downloadReport}>Unduh laporan CSV</button></div>
      {!branch ? <div className="empty-state">Pilih cabang untuk melihat batch.</div> : <ResourceState resource={batches} empty={!items.length} message="Tidak ada batch pada filter ini."><div className="table-scroll"><table>
        <thead><tr><th>Obat (SKU)</th><th>Batch</th><th>ED</th><th>Status ED</th><th>Status</th><th className="num">Jumlah</th><th>Supplier</th><th>Aksi</th></tr></thead>
        <tbody>{items.map(i => { const label = expiryLabel(i.days_to_expiry, warn); return <tr key={`${i.batch_id}-${i.stored_status}`}>
          <td>{i.sku}</td><td>{i.batch_number}</td><td>{i.expiry_date}</td><td><span className={`badge ${label.tone}`}>{label.text}</span></td>
          <td>{i.status === 'QUARANTINE' ? 'Karantina' : i.status === 'EXPIRED' ? 'Kedaluwarsa' : 'Dapat dijual'}</td><td className="num">{formatQty(BigInt(i.qty_milli))}</td><td>{i.last_supplier_name ?? '—'}</td>
          <td className="row"><button disabled={busy} onClick={() => loadTrace(i)}>Telusuri</button>{canManage && <button disabled={busy} onClick={() => setOperation({ kind: 'count', item: i })}>Opname</button>}{canRelease && i.stored_status === 'QUARANTINE' && <button disabled={busy} onClick={() => setOperation({ kind: 'release', item: i })}>Lepas</button>}{canManage && <button disabled={busy} onClick={() => setOperation({ kind: 'dispose', item: i })}>Musnahkan</button>}</td>
        </tr>; })}</tbody></table></div></ResourceState>}
      {batches.data?.truncated && <p className="muted small">Hanya 100 baris pertama ditampilkan. Persempit filter.</p>}
    </section>

    {operation && <section className="panel form-panel"><h2>{operation.kind === 'count' ? 'Opname' : operation.kind === 'release' ? 'Lepas karantina' : 'Musnahkan'} · batch {operation.item.batch_number}</h2><form onSubmit={submitOperation}><fieldset disabled={busy || mutation.blocked}><div className="form-grid">
      {operation.kind === 'count' ? <Field label="Hasil hitung (satuan dasar, boleh 0)" id="counted" name="counted" type="number" min="0" step="1" required /> : <Field label="Jumlah (satuan dasar)" id="qty_action" name="qty" type="number" min="1" step="1" required />}
      <Field label="Alasan (wajib)" id="reason_action"><input id="reason_action" name="reason" required maxLength={200} /></Field>
    </div><div className="row"><button className="primary">Simpan</button><button type="button" onClick={() => setOperation(null)}>Batal</button></div></fieldset></form></section>}

    {trace && <section className="panel"><div className="table-toolbar"><h2>Penelusuran batch {trace.item.batch_number}</h2><button onClick={() => setTrace(null)}>Tutup</button></div>
      <h3>Penerimaan</h3>{!trace.receipts.length ? <p className="muted">Tidak ada.</p> : <div className="table-scroll"><table><thead><tr><th>Faktur</th><th>Status</th><th className="num">Jumlah</th><th>Waktu</th></tr></thead><tbody>{trace.receipts.map(r => <tr key={r.receipt_id}><td>{r.invoice_number || '—'}</td><td>{r.status}</td><td className="num">{formatQty(BigInt(r.qty_milli))}</td><td>{new Date(r.created_at).toLocaleString('id-ID')}</td></tr>)}</tbody></table></div>}
      <h3>Penjualan & retur</h3>{!trace.sales.length ? <p className="muted">Belum pernah terjual.</p> : <div className="table-scroll"><table><thead><tr><th>Struk</th><th>Status</th><th className="num">Terjual</th><th className="num">Diretur</th><th>Override</th></tr></thead><tbody>{trace.sales.map(s => <tr key={s.sale_id}><td>{s.receipt_number}</td><td>{s.status} / {s.return_status}</td><td className="num">{formatQty(BigInt(s.qty_milli))}</td><td className="num">{formatQty(BigInt(s.returned_milli))}</td><td>{s.override_reason || '—'}</td></tr>)}</tbody></table></div>}
    </section>}
  </>;
}
