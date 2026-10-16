import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { formatIDR, formatQty } from '../money.js';
import { localDate } from '../reporting.js';
import { quantityToMilli } from '../cashier.js';
import { batchReportPath } from '../pharmacy.js';
import { PageHeading, Field, Notice, ResourceState } from '../components/UI.jsx';
import { Icon } from '../components/Icon.jsx';

const DRUG_FIELDS = [['active_ingredient', 'Zat aktif/komposisi'], ['dosage_form', 'Bentuk sediaan'], ['strength', 'Kekuatan'], ['manufacturer', 'Produsen'], ['registration_number', 'Nomor izin edar (belum diverifikasi)'], ['classification', 'Golongan (diisi petugas, tidak ditentukan sistem)']];

export default function PharmacyTools() {
  const { businessId, user, business } = useSession();
  const { branch } = useWorkspace();
  if (business?.business_type !== 'pharmacy') return <Notice>Halaman ini hanya untuk bisnis apotek.</Notice>;
  if (!branch) return <Notice>Pilih cabang aktif untuk membuka laporan dan identitas apotek.</Notice>;
  if (!['owner', 'manager', 'warehouse', 'pharmacist'].includes(business?.role)) return <Notice>Halaman pengelolaan ini hanya untuk petugas berizin.</Notice>;
  return <Tools key={`${businessId}.${user?.id}.${branch?.id}`} />;
}

function Tools() {
  const { client, business } = useSession();
  const { branch } = useWorkspace();
  const [tab, setTab] = useState('report');
  const [reorderCursor, setReorderCursor] = useState('');
  const [minimumQuery, setMinimumQuery] = useState('');
  const [from, setFrom] = useState(localDate(branch?.timezone));
  const [to, setTo] = useState(localDate(branch?.timezone));
  const [filter, setFilter] = useState('all');
  const [days, setDays] = useState('90');
  const [cursor, setCursor] = useState('');
  const [drugCursor, setDrugCursor] = useState('');
  const [query, setQuery] = useState('');
  const [batchId, setBatchId] = useState('');
  const [movementCursor, setMovementCursor] = useState('');
  const [editor, setEditor] = useState(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');
  const canEdit = ['owner', 'manager', 'pharmacist'].includes(business?.role);
  const canHistory = ['owner', 'manager', 'warehouse', 'pharmacist'].includes(business?.role);
  const validPeriod = !!from && !!to && from <= to;
  const validDays = /^\d+$/.test(days) && Number(days) <= 3650;
  const reportPath = branch && validPeriod && validDays ? batchReportPath(branch.id, from, to, filter, days) : null;
  const report = useResource(tab === 'report' ? reportPath : null);
  const historyParams = new URLSearchParams({ branch_id: branch?.id ?? '', ...(cursor ? { after: cursor } : {}) });
  const receipts = useResource(tab === 'receipts' && branch && canHistory ? `/pharmacy/batch-receipts?${historyParams}` : null);
  const drugs = useResource(tab === 'drugs' ? `/pharmacy/products${drugCursor ? `?after=${drugCursor}` : ''}` : null);
  const search = useResource(tab === 'drugs' && canEdit && query.trim() ? `/catalog/search?q=${encodeURIComponent(query.trim())}&limit=30` : null);
  const movement = useResource(batchId && branch && canHistory ? `/pharmacy/batches/${batchId}/movements?branch_id=${branch.id}${movementCursor ? `&after=${movementCursor}` : ''}` : null);
  const reorder = useResource(branch ? `/stock/reorder?branch_id=${branch.id}${reorderCursor ? `&after=${reorderCursor}` : ''}` : null);
  const minimumProducts = useResource(tab === 'minimum' && canEdit && minimumQuery.trim() ? `/catalog/search?q=${encodeURIComponent(minimumQuery.trim())}&limit=30` : null);
  const batch = useResource(batchId && branch && canHistory ? `/pharmacy/batches/${batchId}/trace?branch_id=${branch.id}` : null);

  function openBatch(id) { setBatchId(id); setMovementCursor(''); }
  function saveBlob(blob, filename) {
    const url = URL.createObjectURL(blob); const a = document.createElement('a');
    a.href = url; a.download = filename; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  async function download() {
    if (busy || !reportPath) return;
    setBusy(true); setMessage('');
    try { saveBlob(await client.get(`${reportPath}&format=csv`, { responseType: 'blob' }), `stok-batch-${from}-${to}.csv`); }
    catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }
  async function openDrug(id) {
    if (busy) return;
    setBusy(true); setEditor(null); setMessage(''); setSuccess('');
    try {
      const product = await client.get(`/products/${id}`);
      let drug = {};
      try { drug = (await client.get(`/pharmacy/products/${id}`)).drug; }
      catch (error) { if (error.status !== 404) throw error; }
      setEditor({ id, name: product.name, version: product.version, drug });
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }
  async function saveDrug(event) {
    event.preventDefault();
    if (busy || editor.conflict) return;
    const v = Object.fromEntries(new FormData(event.currentTarget));
    const body = Object.fromEntries(DRUG_FIELDS.map(([key]) => [key, String(v[key] ?? '').trim()]));
    body.prescription_required = v.prescription_required === 'on';
    body.special_restrictions = v.special_restrictions === 'on';
    setBusy(true); setMessage(''); setSuccess('');
    try {
      const result = await client.put(`/pharmacy/products/${editor.id}`, body, { version: editor.version });
      setEditor(current => ({ ...current, version: result.version, drug: body }));
      drugs.reload(); setSuccess('Identitas obat disimpan. Sistem tidak memverifikasi klasifikasi atau izin edar.');
    } catch (error) {
      if (error.status === 409) setEditor(current => ({ ...current, conflict: true }));
      setMessage(error.status === 409 ? 'Produk berubah. Muat ulang editor sebelum menyimpan lagi.' : error.message);
    } finally { setBusy(false); }
  }
  async function saveMinimum(event, item = null) {
    event.preventDefault();
    if (busy) return;
    const form = event.currentTarget;
    const values = Object.fromEntries(new FormData(form));
    setBusy(true); setMessage(''); setSuccess('');
    try {
      const variant = item?.variant_id ?? values.variant_id;
      await client.put(`/stock/minimums/${variant}`, { branch_id: branch.id, minimum_qty_milli: quantityToMilli(values.minimum, true) }, { version: item?.version ?? 0 });
      setSuccess('Stok minimum apotek disimpan. Perhitungan memakai batch sellable yang belum kedaluwarsa.');
      if (!item) form.reset();
      reorder.reload();
    } catch (error) { setMessage(error.status === 409 ? 'Minimum sudah berubah/tersedia. Muat ulang daftar dan edit memakai versi terbaru.' : error.message); reorder.reload(); }
    finally { setBusy(false); }
  }
  const nullableMoney = value => value == null ? 'Belum diketahui' : formatIDR(BigInt(value));

  return <div className="pharmacy-tools">
    <PageHeading title="Kelola apotek" description="Pantau persediaan batch, rencanakan pengadaan, dan lengkapi identitas obat."><Link className="button primary" to="/batch"><Icon name="box" size={18} />Operasi batch</Link></PageHeading>
    <Notice>{message}</Notice><Notice kind="success">{success}</Notice>
    <div className="pharmacy-workspace">
    <aside className="pharmacy-sidebar">
      <div className="pharmacy-branch"><span className="pharmacy-symbol"><Icon name="store" size={22} /></span><div><span>Cabang aktif</span><strong>{branch.name}</strong></div></div>
      <div className="pharmacy-nav" role="group" aria-label="Bagian apotek">{[['report', 'Laporan batch', 'Saldo, masa berlaku & nilai', 'chart'], ['minimum', 'Stok minimum', 'Kebutuhan pengadaan obat', 'box'], ...(canHistory ? [['receipts', 'Riwayat penerimaan', 'Telusuri batch yang diterima', 'truck']] : []), ['drugs', 'Identitas obat', 'Komposisi & informasi resep', 'receipt']].map(([key, label, description, icon]) => <button key={key} aria-pressed={tab === key} aria-controls="pharmacy-content" onClick={() => { setTab(key); setMessage(''); setSuccess(''); }}><Icon name={icon} size={20} /><span><strong>{label}</strong><small>{description}</small></span></button>)}</div>
      <p className="pharmacy-sidebar-note">Laporan menggunakan stok batch apotek, terpisah dari stok inti. Klasifikasi obat dan izin edar dicatat oleh petugas.</p>
    </aside>
    <div className="pharmacy-content" id="pharmacy-content">
    {tab === 'minimum' && <section className="panel"><div className="pharmacy-section-heading"><div><h2>Kebutuhan pengadaan</h2><p className="muted small">Pantau ketersediaan dan atur batas minimum setiap produk.</p></div><button onClick={reorder.reload}><Icon name="refresh" size={16} />Muat ulang</button></div><p className="pharmacy-info">Hanya batch layak jual dengan kedaluwarsa setelah tanggal lokal cabang yang dihitung tersedia. Karantina, dimusnahkan, dan kedaluwarsa tidak dihitung.</p>
      <ResourceState resource={reorder} empty={!reorder.data?.items?.length}><div className="table-scroll"><table><thead><tr><th>Produk</th><th>Tersedia/minimum</th><th>Supplier batch terakhir</th>{canEdit && <th>Edit minimum</th>}</tr></thead><tbody>{reorder.data?.items?.map(item => <tr key={item.variant_id}><td>{item.product_name} · {item.sku}</td><td>{formatQty(BigInt(item.available_milli))} / {formatQty(BigInt(item.minimum_qty_milli))}</td><td>{item.last_supplier_name ?? '—'}</td>{canEdit && <td><form key={`${item.variant_id}-${item.version}`} onSubmit={event => saveMinimum(event, item)}><input name="minimum" aria-label={`Minimum ${item.product_name}`} type="number" min="0" step="0.001" required defaultValue={formatQty(BigInt(item.minimum_qty_milli)).replace(',', '.')} disabled={busy} /><button disabled={busy}>Simpan</button></form></td>}</tr>)}</tbody></table></div></ResourceState>
      <div className="row"><button disabled={!reorderCursor || busy} onClick={() => setReorderCursor('')}>Halaman pertama</button><button disabled={busy || reorder.loading || !reorder.data?.next_cursor} onClick={() => setReorderCursor(reorder.data.next_cursor)}>Kebutuhan berikutnya</button></div>
      {canEdit && <><h3>Buat minimum baru</h3><Field label="Cari produk apotek" id="minimum_product_search" value={minimumQuery} onChange={event => setMinimumQuery(event.target.value)} /><ResourceState resource={minimumProducts}><form onSubmit={saveMinimum}><fieldset disabled={busy}><label>Produk<select name="variant_id" required><option value="">Pilih produk terlacak stok</option>{minimumProducts.data?.items?.filter(hit => hit.track_stock).map(hit => <option key={hit.variant.id} value={hit.variant.id}>{hit.product_name} · {hit.variant.sku}</option>)}</select></label><label>Minimum (satuan dasar)<input name="minimum" type="number" min="0" step="0.001" required /></label><button>Buat minimum</button></fieldset></form></ResourceState></>}
    </section>}
    {tab === 'report' && <section className="panel"><div className="pharmacy-section-heading"><div><h2>Laporan persediaan batch</h2><p className="muted small">Periksa saldo, masa kedaluwarsa, dan pergerakan obat.</p></div><span className="badge">Saldo saat ini</span></div>
      <div className="pharmacy-filters"><div className="form-grid"><Field label="Pergerakan dari" id="report_from" type="date" required value={from} max={to} onChange={e => setFrom(e.target.value)} /><Field label="Sampai tanggal" id="report_to" type="date" required value={to} min={from} onChange={e => setTo(e.target.value)} />
        <Field label="Filter batch" id="report_filter"><select id="report_filter" value={filter} onChange={e => setFilter(e.target.value)}><option value="all">Semua</option><option value="near_expiry">Mendekati ED</option><option value="expired">Kedaluwarsa</option><option value="quarantine">Karantina</option></select></Field><Field label="Ambang mendekati ED (hari)" id="days" type="number" min="0" max="3650" value={days} onChange={e => setDays(e.target.value)} /></div>
      {(!validPeriod || !validDays) && <Notice>Periksa periode dan ambang hari (0–3650).</Notice>}
      <div className="pharmacy-actions"><button className="primary" disabled={busy || !reportPath || report.loading || !!report.error} onClick={download}><Icon name="receipt" size={17} />{busy ? 'Menyiapkan CSV…' : 'Unduh laporan CSV'}</button><button onClick={report.reload} disabled={!reportPath || report.loading}><Icon name="refresh" size={17} />Muat ulang</button></div></div>
      <p className="pharmacy-info"><Icon name="clock" size={18} />Periode menyaring pergerakan saja. Saldo dan nilai menampilkan kondisi saat ini, bukan saldo historis.</p>
      <h3 className="pharmacy-data-heading">Rincian batch</h3>
      <ResourceState resource={report} empty={!report.data?.rows?.length}><p className="muted small">{report.data?.valuation_label}</p><div className="table-scroll"><table><thead><tr><th>Produk/SKU</th><th>Batch</th><th>ED</th><th>Status</th><th>Saldo</th><th>Pergerakan periode</th><th>Supplier terakhir</th>{report.data?.cost_visible && <><th>Biaya/unit</th><th>Estimasi nilai</th></>}<th>Detail</th></tr></thead><tbody>{report.data?.rows?.map(r => <tr key={`${r.batch_id}-${r.stored_status}`}><td>{r.product_name} · {r.sku}</td><td>{r.batch_number}</td><td>{r.expiry_date} ({r.days_to_expiry} hari)</td><td>{r.status}</td><td>{formatQty(BigInt(r.qty_milli))}</td><td>{formatQty(BigInt(r.period_movement_milli))} ({r.period_movement_count} gerakan)</td><td>{r.last_supplier_name ?? '—'}</td>{report.data?.cost_visible && <><td>{nullableMoney(r.average_cost_cents)}</td><td>{nullableMoney(r.estimated_value_cents)}</td></>}<td>{canHistory && <button onClick={() => openBatch(r.batch_id)}>Kartu batch</button>}</td></tr>)}</tbody></table></div></ResourceState>
    </section>}
    {tab === 'receipts' && canHistory && <section className="panel"><h2>Riwayat penerimaan batch cabang</h2><ResourceState resource={receipts} empty={!receipts.data?.receipts?.length}><div className="table-scroll"><table><thead><tr><th>Faktur</th><th>Batch/ED</th><th>Status</th><th>Jumlah</th><th>Detail</th></tr></thead><tbody>{receipts.data?.receipts?.map(r => <tr key={r.receipt_id}><td>{r.invoice_number || '—'}</td><td>{r.batch_number} · {r.expiry_date}</td><td>{r.status}</td><td>{formatQty(BigInt(r.qty_milli))}</td><td><button onClick={() => openBatch(r.batch_id)}>Telusuri</button></td></tr>)}</tbody></table></div></ResourceState><div className="row"><button disabled={!cursor} onClick={() => setCursor('')}>Halaman pertama</button><button disabled={receipts.loading || !receipts.data?.next_cursor} onClick={() => setCursor(receipts.data.next_cursor)}>Berikutnya</button></div></section>}
    {tab === 'drugs' && <section className="panel"><h2>Identitas obat</h2>{canEdit && <><Field label="Cari produk/SKU untuk menambah identitas" id="drug_search" value={query} onChange={e => setQuery(e.target.value)} /><ResourceState resource={search}>{search.data?.items?.map(hit => <button key={hit.variant.id} disabled={busy} onClick={() => openDrug(hit.product_id)}>{hit.product_name} · {hit.variant.sku}</button>)}</ResourceState></>}
      <ResourceState resource={drugs} empty={!drugs.data?.drugs?.length}><div className="table-scroll"><table><thead><tr><th>Produk</th><th>Golongan</th><th>Wajib resep</th><th>Pembatasan khusus</th><th>Aksi</th></tr></thead><tbody>{drugs.data?.drugs?.map(d => <tr key={d.product_id}><td>{d.name}</td><td>{d.classification || 'Belum dicatat'}</td><td>{d.prescription_required ? 'Ya' : 'Tidak'}</td><td>{d.special_restrictions ? 'Ya' : 'Tidak'}</td><td>{canEdit && <button disabled={busy} onClick={() => openDrug(d.product_id)}>Edit identitas</button>}</td></tr>)}</tbody></table></div></ResourceState><div className="row"><button disabled={!drugCursor} onClick={() => setDrugCursor('')}>Halaman pertama</button><button disabled={drugs.loading || !drugs.data?.next_cursor} onClick={() => setDrugCursor(drugs.data.next_cursor)}>Berikutnya</button></div>
      {editor && <form key={`${editor.id}-${editor.version}`} onSubmit={saveDrug}><h3>{editor.name}</h3><fieldset disabled={busy || editor.conflict}><div className="form-grid">{DRUG_FIELDS.map(([key, label]) => <Field key={key} label={label} id={`drug_${key}`}><input id={`drug_${key}`} name={key} maxLength={500} defaultValue={editor.drug[key] ?? ''} /></Field>)}</div><label><input name="prescription_required" type="checkbox" defaultChecked={editor.drug.prescription_required} /> Wajib referensi resep</label><label><input name="special_restrictions" type="checkbox" defaultChecked={editor.drug.special_restrictions} /> Pembatasan khusus</label><button className="primary">Simpan identitas</button></fieldset><div className="row"><button type="button" disabled={busy} onClick={() => openDrug(editor.id)}>Muat ulang versi produk</button><button type="button" disabled={busy} onClick={() => setEditor(null)}>Tutup editor</button></div></form>}
    </section>}
    {batchId && <section className="panel"><div className="table-toolbar"><h2>Kartu batch</h2><button onClick={() => setBatchId('')}>Tutup</button></div><ResourceState resource={movement} empty={!movement.data?.movements?.length}><div className="table-scroll"><table><thead><tr><th>Waktu</th><th>Jenis/status</th><th>Perubahan</th><th>Saldo sesudah</th><th>Alasan</th><th>Referensi</th></tr></thead><tbody>{movement.data?.movements?.map(m => <tr key={m.id}><td>{new Date(m.created_at).toLocaleString('id-ID', { timeZone: branch.timezone })}</td><td>{m.kind} / {m.status}</td><td>{formatQty(BigInt(m.qty_milli))}</td><td>{formatQty(BigInt(m.balance_after_milli))}</td><td>{m.reason || '—'}</td><td>{m.ref_id}</td></tr>)}</tbody></table></div></ResourceState><div className="row"><button disabled={!movementCursor} onClick={() => setMovementCursor('')}>Halaman pertama</button><button disabled={movement.loading || !movement.data?.next_cursor} onClick={() => setMovementCursor(movement.data.next_cursor)}>Gerakan berikutnya</button></div><ResourceState resource={batch}><h3>Penjualan dan retur batch asal</h3>{batch.data?.sales?.map((sale, index) => <p key={`${sale.sale_id}-${index}`}>{sale.receipt_number} · {sale.status}/{sale.return_status} · terjual {formatQty(BigInt(sale.qty_milli))}, diretur {formatQty(BigInt(sale.returned_milli))}{sale.override_reason ? ` · override: ${sale.override_reason}` : ''}</p>)}<p className="muted small">Penelusuran penjualan dibatasi server 200 baris; kartu pergerakan memiliki pagination.</p></ResourceState></section>}
    </div>
    </div>
  </div>;
}
