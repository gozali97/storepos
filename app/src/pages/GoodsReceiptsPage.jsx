import { useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { usePendingMutation } from '../hooks/usePendingMutation.js';
import { newIdempotencyKey } from '../api/client.js';
import { receiptDraftBody, receiptEditableLines } from '../purchasing.js';
import { localDate } from '../reporting.js';
import { formatQty, formatIDR } from '../money.js';
import { PageHeading, Notice, ResourceState } from '../components/UI.jsx';

export default function GoodsReceiptsPage() {
  const { businessId, user } = useSession();
  const { branch } = useWorkspace();
  return <Receipts key={`${businessId}.${user?.id}.${branch?.id}`} />;
}

function Receipts() {
  const { client, businessId, user, business } = useSession();
  const { branch } = useWorkspace();
  const [from, setFrom] = useState(localDate(branch?.timezone));
  const [to, setTo] = useState(localDate(branch?.timezone));
  const [status, setStatus] = useState('');
  const [cursor, setCursor] = useState('');
  const [invoice, setInvoice] = useState('');
  const [selected, setSelected] = useState(null);
  const [lines, setLines] = useState([]);
  const [units, setUnits] = useState({});
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [saved, setSaved] = useState(false);
  const pending = usePendingMutation(`storepos.pendingReceiptPost.${businessId}.${user?.id}.${branch?.id}`);
  const params = new URLSearchParams({ branch_id: branch?.id ?? '', from, to, status, invoice_number: invoice, ...(cursor ? { before: cursor } : {}) });
  const list = useResource(branch && from && to && from <= to ? `/goods-receipts?${params}` : null);
  const suppliers = useResource('/suppliers');
  const canWrite = ['owner', 'manager', 'warehouse', 'pharmacist'].includes(business?.role);

  async function load(id) {
    if (busy || pending.blocked) return;
    setBusy(true); setMessage('');
    try {
      const data = await client.get(`/goods-receipts/${id}`);
      if (data.branch_id !== branch.id) throw new Error('Dokumen bukan cabang aktif.');
      const ids = [...new Set(data.lines.map(line => line.variant_id))];
      const fetched = await Promise.all(ids.map(async variant => [variant, (await client.get(`/variants/${variant}/units`)).units ?? []]));
      setSelected(data); setLines(receiptEditableLines(data)); setUnits(Object.fromEntries(fetched)); setSaved(false);
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }

  async function save(event) {
    event.preventDefault();
    if (busy || pending.blocked) return;
    const v = Object.fromEntries(new FormData(event.currentTarget));
    setBusy(true); setMessage('');
    try {
      await client.put(`/goods-receipts/${selected.receipt_id}`, receiptDraftBody(v, lines), { version: selected.version });
      const latest = await client.get(`/goods-receipts/${selected.receipt_id}`);
      setSelected(latest); setLines(receiptEditableLines(latest)); setSaved(true); list.reload();
    } catch (error) { setSaved(false); setMessage(`${error.message} Muat ulang dokumen sebelum menyimpan ulang.`); }
    finally { setBusy(false); }
  }

  async function post(operation) {
    if (busy) return;
    setBusy(true); setMessage('');
    try {
      const result = await pending.send(operation);
      setSelected(current => current ? { ...current, ...result } : result);
      setSaved(false); list.reload();
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }

  const update = (index, patch) => { setSaved(false); setLines(rows => rows.map((row, i) => i === index ? { ...row, ...patch } : row)); };
  return <>
    <PageHeading title="Dokumen penerimaan" description="Cari draft, periksa/edit baris, lalu posting. Riwayat posted tidak dapat diedit." />
    <Notice>{message || pending.error}</Notice>
    {pending.pending && <div className="alert warn" role="status">Status posting belum pasti. Jangan buat dokumen baru untuk penerimaan yang sama.<button disabled={busy || pending.sending} onClick={() => post(pending.pending)}>Periksa & coba lagi (key asli)</button></div>}
    <section className="panel"><fieldset disabled={busy || pending.blocked}><div className="filter-fields">
      <label>Dari<input type="date" value={from} max={to} onChange={e => { setFrom(e.target.value); setCursor(''); }} /></label>
      <label>Sampai<input type="date" value={to} min={from} onChange={e => { setTo(e.target.value); setCursor(''); }} /></label>
      <label>Status<select value={status} onChange={e => { setStatus(e.target.value); setCursor(''); }}><option value="">Semua</option><option value="draft">Draft</option><option value="posted">Posted</option></select></label>
      <label>Nomor faktur (persis)<input value={invoice} maxLength={200} onChange={e => { setInvoice(e.target.value); setCursor(''); }} /></label>
    </div></fieldset>
      <ResourceState resource={list} empty={!list.data?.receipts?.length}><div className="table-scroll"><table><thead><tr><th>Tanggal</th><th>Faktur</th><th>Supplier</th><th>Status</th><th>Aksi</th></tr></thead><tbody>{list.data?.receipts?.map(r => <tr key={r.receipt_id}><td>{r.received_date}</td><td>{r.invoice_number || '—'}</td><td>{r.supplier_name || '—'}</td><td>{r.status}</td><td><button disabled={busy || pending.blocked} onClick={() => load(r.receipt_id)}>Buka</button></td></tr>)}</tbody></table></div></ResourceState>
      <div className="row"><button disabled={busy || pending.blocked || !cursor} onClick={() => setCursor('')}>Halaman pertama</button><button disabled={busy || pending.blocked || !list.data?.next_cursor} onClick={() => setCursor(list.data.next_cursor)}>Berikutnya</button></div>
    </section>
    {selected && <section className="panel form-panel"><h2>{selected.status === 'posted' ? 'Penerimaan posted' : 'Edit draft'} · {selected.receipt_id}</h2>
      <button disabled={busy || pending.blocked} onClick={() => load(selected.receipt_id)}>Muat ulang status server</button>
      {selected.status === 'draft' && selected.lines ? <>
        <form key={`${selected.receipt_id}-${selected.version}`} onSubmit={save}><fieldset disabled={busy || pending.blocked || !canWrite}>
          <div className="form-grid"><label>Tanggal<input name="received_date" type="date" required defaultValue={selected.received_date} onChange={() => setSaved(false)} /></label><label>Faktur<input name="invoice_number" maxLength={200} defaultValue={selected.invoice_number} onChange={() => setSaved(false)} /></label><label>Supplier<select name="supplier_id" defaultValue={selected.supplier_id ?? ''} onChange={() => setSaved(false)}><option value="">Tanpa supplier</option>{selected.supplier_id && !suppliers.data?.suppliers?.some(s => s.id === selected.supplier_id) && <option value={selected.supplier_id}>Supplier dokumen saat ini</option>}{suppliers.data?.suppliers?.map(s => <option key={s.id} value={s.id}>{s.name}{s.archived ? ' (arsip)' : ''}</option>)}</select></label></div>
          {lines.map((line, index) => <div className="form-grid" key={`${selected.receipt_id}-${index}`}><label>Satuan<select value={line.unit_id} onChange={e => update(index, { unit_id: e.target.value })}>{!units[line.variant_id]?.some(u => u.unit_id === line.unit_id) && <option value={line.unit_id}>Satuan asli dokumen</option>}{units[line.variant_id]?.map(u => <option key={u.unit_id} value={u.unit_id}>{u.name} ({formatQty(BigInt(u.factor_milli))} satuan dasar)</option>)}</select></label><label>Jumlah<input type="number" min="0.001" step="0.001" required value={line.qty} onChange={e => update(index, { qty: e.target.value })} /></label><label>Harga per satuan (Rp)<input type="number" min="0" step="0.01" required value={line.price} onChange={e => update(index, { price: e.target.value })} /></label><button type="button" disabled={lines.length === 1} onClick={() => { setSaved(false); setLines(rows => rows.filter((_, i) => i !== index)); }}>Hapus baris</button></div>)}
          <button className="primary">Simpan draft</button>
        </fieldset></form>
        <p className="muted">Simpan draft sebelum posting agar angka yang dikirim persis data terbaru.</p>
        <button className="primary" disabled={busy || pending.blocked || !saved || !canWrite} onClick={() => post({ path: `/goods-receipts/${selected.receipt_id}/post`, body: {}, version: selected.version, key: newIdempotencyKey() })}>Posting penerimaan</button>
      </> : <div><p>Stok sudah diposting; tidak perlu mengirim ulang.</p>{selected.lines?.map(line => <p key={line.line_id}>{line.unit_name ?? line.unit_id} · {formatQty(BigInt(line.qty_milli))} · {formatIDR(BigInt(line.purchase_price_cents))}</p>)}</div>}
    </section>}
  </>;
}
