import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useSession } from '../auth/SessionContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { discountToInput } from '../cashier.js';
import { taxRuleBody } from '../taxes.js';
import { PageHeading, Field, Notice, ResourceState } from '../components/UI.jsx';

export default function TaxSettingsPage() {
  const { businessId, business, user } = useSession();
  if (business?.role !== 'owner') return <Notice>Aturan dan penugasan pajak hanya dapat dikelola Pemilik.</Notice>;
  return <TaxWorkspace key={`${businessId}.${user?.id}`} />;
}

function TaxWorkspace() {
  const { client } = useSession();
  const [cursor, setCursor] = useState('');
  const [editing, setEditing] = useState(null);
  const [confirming, setConfirming] = useState(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');
  const [query, setQuery] = useState('');
  const [target, setTarget] = useState(null);
  const [assignment, setAssignment] = useState(null);
  const [assignedRule, setAssignedRule] = useState(null);
  const [assignmentError, setAssignmentError] = useState('');
  const [pickedRule, setPickedRule] = useState('');
  const rules = useResource(`/tax-rules${cursor ? `?after=${cursor}` : ''}`);
  const defaultTax = useResource('/settings/tax');
  const categories = useResource('/categories');
  const products = useResource(query.trim() ? `/catalog/search?q=${encodeURIComponent(query.trim())}&limit=30` : null);
  const productChoices = [...new Map((products.data?.items ?? []).map(hit => [hit.product_id, hit])).values()];
  const percent = value => discountToInput({ kind: 'percent', value: String(value) });

  async function saveRule(event) {
    event.preventDefault();
    if (busy || editing?.conflict) return;
    const v = Object.fromEntries(new FormData(event.currentTarget));
    setBusy(true); setMessage(''); setSuccess('');
    try {
      const body = taxRuleBody(v.name, v.rate, v.mode);
      if (editing?.id) await client.put(`/tax-rules/${editing.id}`, body, { version: editing.version });
      else await client.post('/tax-rules', body);
      setEditing(null); rules.reload(); setSuccess('Aturan pajak disimpan untuk transaksi berikutnya.');
    } catch (error) {
      if (error.status === 409 && editing?.id) setEditing(current => ({ ...current, conflict: true }));
      setMessage(error.message);
    } finally { setBusy(false); }
  }
  async function editRule(id) {
    if (busy) return;
    setBusy(true); setMessage('');
    try { setEditing(await client.get(`/tax-rules/${id}`)); }
    catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }
  async function archiveRule() {
    if (busy || !confirming) return;
    setBusy(true); setMessage(''); setSuccess('');
    try {
      await client.del(`/tax-rules/${confirming.id}`, { version: confirming.version });
      setConfirming(null); rules.reload(); setSuccess('Aturan diarsipkan. Riwayat transaksi tetap memakai snapshot lama.');
    } catch (error) { setMessage(error.message); rules.reload(); setConfirming(null); }
    finally { setBusy(false); }
  }
  async function restoreRule(rule) {
    if (busy) return;
    setBusy(true); setMessage(''); setSuccess('');
    try { await client.post(`/tax-rules/${rule.id}/restore`, {}, { version: rule.version }); rules.reload(); setSuccess('Aturan dipulihkan.'); }
    catch (error) { setMessage(error.message); rules.reload(); }
    finally { setBusy(false); }
  }
  async function selectTarget(kind, id, name) {
    if (busy) return;
    setBusy(true); setMessage(''); setTarget({ kind, id, name }); setAssignment(null); setAssignedRule(null); setAssignmentError('');
    try {
      const data = await client.get(`/${kind}/${id}/tax-rule`);
      setAssignment(data); setPickedRule(data.tax_rule_id ?? '');
      if (data.tax_rule_id) setAssignedRule(await client.get(`/tax-rules/${data.tax_rule_id}`));
    } catch (error) { setAssignmentError(error.message); }
    finally { setBusy(false); }
  }
  async function assign(event) {
    event.preventDefault();
    if (busy || !assignment || assignmentError) return;
    setBusy(true); setMessage(''); setSuccess('');
    try {
      const result = await client.put(`/${target.kind}/${target.id}/tax-rule`, { tax_rule_id: pickedRule || null }, { version: assignment.version });
      setAssignment(result); setAssignedRule(null);
      setSuccess(`${target.name}: ${pickedRule ? 'aturan pajak disimpan' : 'penugasan dilepas; kembali ke prioritas berikutnya'}.`);
    } catch (error) {
      setMessage(error.message);
      if (error.status === 409) setAssignmentError('Versi master berubah. Muat ulang penugasan sebelum mengulang.');
    } finally { setBusy(false); }
  }
  async function saveDefault(event) {
    event.preventDefault();
    if (busy) return;
    const v = Object.fromEntries(new FormData(event.currentTarget));
    setBusy(true); setMessage(''); setSuccess('');
    try { await client.put('/settings/tax', taxRuleBody(v.name, v.rate, v.mode)); defaultTax.reload(); setSuccess('Pajak default bisnis diperbarui.'); }
    catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }

  return <>
    <PageHeading title="Pajak & penugasan" description="Prioritas: aturan produk → aturan kategori → pajak default bisnis. Transaksi lama memakai snapshot."><Link to="/pengaturan">Pengaturan</Link></PageHeading>
    <Notice>{message}</Notice><Notice kind="success">{success}</Notice>
    <section className="panel form-panel"><h2>Pajak default bisnis</h2><ResourceState resource={defaultTax}>{defaultTax.data && <form key={JSON.stringify(defaultTax.data)} onSubmit={saveDefault}><fieldset disabled={busy}>
      <Field label="Nama pajak default" id="default_tax_name"><input name="name" id="default_tax_name" required maxLength={100} defaultValue={defaultTax.data.rule?.name ?? 'Tanpa pajak'} /></Field>
      <Field label="Tarif default (%; nol untuk 0%)" id="default_tax_rate"><input name="rate" id="default_tax_rate" type="number" min="0" max="100" step="0.01" required defaultValue={percent(defaultTax.data.rule?.rate_bps ?? 0)} /></Field>
      <Field label="Mode default" id="default_tax_mode"><select name="mode" id="default_tax_mode" defaultValue={defaultTax.data.rule?.mode ?? 'exclusive'}><option value="exclusive">Eksklusif — pajak ditambahkan</option><option value="inclusive">Inklusif — pajak sudah termasuk harga</option></select></Field>
      <p className="muted">Perubahan hanya berlaku untuk transaksi berikutnya; cocokkan tarif dengan kebijakan usaha Anda.</p><button className="primary">Simpan default</button>
    </fieldset></form>}</ResourceState></section>
    <section className="panel"><div className="table-toolbar"><h2>Aturan produk/kategori</h2><button disabled={busy} onClick={() => setEditing({})}>Aturan baru</button><button disabled={busy} onClick={rules.reload}>Muat ulang</button></div>
      {editing && <form key={`${editing.id ?? 'new'}-${editing.version ?? 0}`} onSubmit={saveRule}><fieldset disabled={busy || editing.conflict}>
        <Field label="Nama aturan" id="rule_name"><input name="name" id="rule_name" required maxLength={100} defaultValue={editing.name ?? ''} /></Field>
        <Field label="Tarif (%)" id="rule_rate"><input name="rate" id="rule_rate" type="number" min="0" max="100" step="0.01" required defaultValue={percent(editing.rate_bps ?? 0)} /></Field>
        <Field label="Mode pajak" id="rule_mode"><select name="mode" id="rule_mode" defaultValue={editing.mode ?? 'exclusive'}><option value="exclusive">Eksklusif</option><option value="inclusive">Inklusif</option></select></Field>
        <button className="primary">Simpan aturan</button>
      </fieldset><div className="row">{editing.id && <button type="button" disabled={busy} onClick={() => editRule(editing.id)}>Muat ulang versi</button>}<button type="button" disabled={busy} onClick={() => setEditing(null)}>Batal</button></div></form>}
      {confirming && <div className="alert warn" role="status">Arsipkan {confirming.name}? Backend menolak jika masih ditugaskan ke produk/kategori.<button disabled={busy} onClick={archiveRule}>Konfirmasi arsip</button><button disabled={busy} onClick={() => setConfirming(null)}>Batal</button></div>}
      <ResourceState resource={rules} empty={!rules.data?.rules?.length}><div className="table-scroll"><table><thead><tr><th>Aturan</th><th>Tarif</th><th>Mode</th><th>Status</th><th>Aksi</th></tr></thead><tbody>{rules.data?.rules?.map(rule => <tr key={rule.id}><td>{rule.name}</td><td>{percent(rule.rate_bps)}%</td><td>{rule.mode}</td><td>{rule.archived ? 'Arsip' : 'Aktif'}</td><td>{rule.archived ? <button disabled={busy} onClick={() => restoreRule(rule)}>Pulihkan</button> : <div className="row"><button disabled={busy} onClick={() => editRule(rule.id)}>Edit</button><button disabled={busy} onClick={() => setConfirming(rule)}>Arsipkan</button></div>}</td></tr>)}</tbody></table></div></ResourceState>
      <div className="row"><button disabled={busy || !cursor} onClick={() => setCursor('')}>Halaman pertama</button><button disabled={busy || rules.loading || !rules.data?.next_cursor} onClick={() => setCursor(rules.data.next_cursor)}>Aturan berikutnya</button></div>
    </section>
    <section className="panel"><h2>Penugasan pajak</h2><p>Pilih produk atau kategori. Kosongkan aturan untuk kembali ke kategori/default bisnis, bukan untuk membebaskan pajak secara otomatis. Gunakan aturan 0% bila memang diperlukan.</p>
      <Field label="Cari nama/SKU produk" id="tax_product_search" value={query} onChange={e => setQuery(e.target.value)} />
      <ResourceState resource={products}>{productChoices.map(hit => <button key={hit.product_id} disabled={busy} onClick={() => selectTarget('products', hit.product_id, hit.product_name)}>{hit.product_name}</button>)}</ResourceState>
      <ResourceState resource={categories}><div className="row">{categories.data?.categories?.map(c => <button key={c.id} disabled={busy} onClick={() => selectTarget('categories', c.id, c.name)}>Kategori: {c.name}</button>)}</div></ResourceState>
      {target && <div><h3>{target.kind === 'products' ? 'Produk' : 'Kategori'}: {target.name}</h3><Notice>{assignmentError}</Notice>
        {assignment && <form key={`${target.kind}-${target.id}-${assignment.version}`} onSubmit={assign}><fieldset disabled={busy || !!assignmentError}>
          <Field label="Aturan aktif" id="assigned_tax"><select id="assigned_tax" value={pickedRule} onChange={e => setPickedRule(e.target.value)}><option value="">Warisi kategori/default bisnis</option>{pickedRule && !rules.data?.rules?.some(r => r.id === pickedRule) && <option value={pickedRule}>{assignedRule?.name ?? `Aturan saat ini (${pickedRule})`}</option>}{rules.data?.rules?.filter(r => !r.archived || r.id === pickedRule).map(r => <option key={r.id} value={r.id} disabled={r.archived}>{r.name} · {percent(r.rate_bps)}%{r.archived ? ' (arsip)' : ''}</option>)}</select></Field>
          <p className="muted small">Pilihan mengikuti halaman daftar aturan saat ini. Gunakan Aturan berikutnya untuk memuat pilihan lainnya.</p><button className="primary">Simpan penugasan</button>
        </fieldset></form>}
        <div className="row"><button disabled={busy} onClick={() => selectTarget(target.kind, target.id, target.name)}>Muat ulang penugasan</button><button disabled={busy} onClick={() => { setTarget(null); setAssignment(null); setAssignedRule(null); setAssignmentError(''); }}>Tutup penugasan</button></div>
      </div>}
    </section>
  </>;
}
