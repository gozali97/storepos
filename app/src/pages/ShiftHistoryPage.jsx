import { useEffect, useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { usePendingMutation } from '../hooks/usePendingMutation.js';
import { newIdempotencyKey } from '../api/client.js';
import { localDate } from '../reporting.js';
import { formatIDR } from '../money.js';
import { PageHeading, Notice, ResourceState } from '../components/UI.jsx';

export default function ShiftHistoryPage() {
  const { businessId, business, user } = useSession();
  const { branch } = useWorkspace();
  if (!['owner', 'manager', 'cashier', 'pharmacist'].includes(business?.role)) return <Notice>Riwayat shift hanya untuk peran berizin.</Notice>;
  return <History key={`${businessId}.${user?.id}.${branch?.id}`} />;
}

function History() {
  const { businessId, business, user } = useSession();
  const { branch } = useWorkspace();
  const [from, setFrom] = useState(localDate(branch?.timezone));
  const [to, setTo] = useState(localDate(branch?.timezone));
  const [status, setStatus] = useState('all');
  const [register, setRegister] = useState('');
  const [cursor, setCursor] = useState('');
  const [selected, setSelected] = useState('');
  const [noteCursor, setNoteCursor] = useState('');
  const [body, setBody] = useState('');
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');
  const pending = usePendingMutation(`storepos.shiftNotes.${businessId}.${user?.id}.${branch?.id}`);
  const canNote = ['owner', 'manager'].includes(business?.role);
  const params = new URLSearchParams({ branch_id: branch?.id ?? '', from, to, status, ...(register ? { register_id: register } : {}), ...(cursor ? { before: cursor } : {}) });
  const shifts = useResource(branch && from && to && from <= to ? `/shifts?${params}` : null);
  const registers = useResource(branch ? `/branches/${branch.id}/registers?status=all` : null);
  const summary = useResource(selected ? `/shifts/${selected}/summary` : null);
  const notes = useResource(selected ? `/shifts/${selected}/notes${noteCursor ? `?after=${noteCursor}` : ''}` : null);
  useEffect(() => () => document.body.classList.remove('print-shift-summary'), []);
  const money = value => value == null ? '—' : formatIDR(BigInt(value));
  const time = value => value ? new Date(value).toLocaleString('id-ID', { timeZone: branch?.timezone ?? 'Asia/Jakarta' }) : '—';
  function choose(id) { setSelected(id); setNoteCursor(''); setBody(''); setMessage(''); setSuccess(''); }
  async function send(operation) {
    setMessage(''); setSuccess('');
    try {
      await pending.send(operation);
      setBody(''); setSuccess('Catatan koreksi ditambahkan tanpa mengubah angka shift.'); notes.reload();
    } catch (error) { setMessage(error.message); }
  }
  function addNote(event) {
    event.preventDefault();
    if (pending.blocked || !selected) return;
    const text = body.trim();
    if (!text || [...text].length > 1000) { setMessage('Catatan wajib maksimal 1000 karakter.'); return; }
    send({ path: `/shifts/${selected}/notes`, body: { body: text }, key: newIdempotencyKey() });
  }
  function print() {
    document.body.classList.add('print-shift-summary');
    const cleanup = () => document.body.classList.remove('print-shift-summary');
    window.addEventListener('afterprint', cleanup, { once: true });
    try { window.print(); } catch (error) { cleanup(); setMessage(error.message); }
  }

  return <>
    <PageHeading title="Riwayat & koreksi shift" description="Ringkasan hanya untuk shift tertutup; catatan koreksi tidak mengubah saldo, hitungan atau status final." />
    <Notice>{message || pending.error}</Notice><Notice kind="success">{success}</Notice>
    {pending.pending && <div className="alert warn" role="status">Status penambahan catatan belum pasti.<button disabled={pending.sending} onClick={() => send(pending.pending)}>Periksa & coba lagi dengan key asli</button></div>}
    <section className="panel"><fieldset disabled={pending.blocked}><div className="filter-fields">
      <label>Dari<input type="date" value={from} max={to} onChange={e => { setFrom(e.target.value); setCursor(''); }} /></label>
      <label>Sampai<input type="date" value={to} min={from} onChange={e => { setTo(e.target.value); setCursor(''); }} /></label>
      <label>Status<select value={status} onChange={e => { setStatus(e.target.value); setCursor(''); }}><option value="all">Semua</option><option value="open">Terbuka</option><option value="closed">Tertutup</option></select></label>
      <label>Terminal<select value={register} onChange={e => { setRegister(e.target.value); setCursor(''); }}><option value="">Semua terminal</option>{registers.data?.registers?.map(r => <option key={r.register_id} value={r.register_id}>{r.name} ({r.status})</option>)}</select></label>
    </div></fieldset>
    {!branch ? <p>Pilih cabang.</p> : <ResourceState resource={shifts} empty={!shifts.data?.shifts?.length}><p className="muted">Cakupan: {shifts.data?.scope === 'branch' ? 'Semua petugas pada cabang berizin' : 'Shift milik Anda saja'}.</p><div className="table-scroll"><table><thead><tr><th>Terminal/petugas</th><th>Dibuka</th><th>Ditutup</th><th>Status</th><th>Kas diharapkan</th><th>Terhitung</th><th>Selisih</th><th>Aksi</th></tr></thead><tbody>{shifts.data?.shifts?.map(s => <tr key={s.shift_id}><td>{s.register_name} · {s.cashier_name}</td><td>{time(s.opened_at)}</td><td>{time(s.closed_at)}</td><td>{s.status}</td><td>{money(s.expected_cash_cents)}</td><td>{money(s.counted_cash_cents)}</td><td>{money(s.variance_cents)}</td><td>{s.status === 'closed' ? <button disabled={pending.blocked} onClick={() => choose(s.shift_id)}>Ringkasan & catatan</button> : 'Hitung buta belum selesai'}</td></tr>)}</tbody></table></div></ResourceState>}
    <div className="row"><button disabled={pending.blocked || !cursor} onClick={() => setCursor('')}>Halaman pertama</button><button disabled={pending.blocked || shifts.loading || !shifts.data?.next_cursor} onClick={() => setCursor(shifts.data.next_cursor)}>Berikutnya</button><button disabled={shifts.loading} onClick={shifts.reload}>Muat ulang</button></div></section>
    {selected && <section className="panel print-shift-target"><h2>Ringkasan shift tertutup</h2><p>ID {selected}</p><ResourceState resource={summary}>{summary.data && <>
      {[
        ['Kas awal', 'opening_cash_cents'], ['Kas diharapkan', 'expected_cash_cents'], ['Kas terhitung', 'counted_cash_cents'], ['Selisih', 'variance_cents'], ['Kas masuk', 'cash_in_cents'], ['Kas keluar', 'cash_out_cents'], ['Refund tunai', 'cash_refund_cents'], ['Diskon item', 'line_discount_cents'], ['Diskon transaksi', 'transaction_discount_cents'],
      ].map(([label,key]) => <div key={key}><span>{label}</span><strong>{money(summary.data[key])}</strong></div>)}
      <p>Penjualan: {summary.data.sale_count}; void: {summary.data.void_count}</p><h3>Pembayaran</h3><div className="table-scroll"><table><thead><tr><th>Metode/verifikasi</th><th>Neto</th><th>Diserahkan</th><th>Kembalian</th></tr></thead><tbody>{summary.data.payments.map(p => <tr key={`${p.method}-${p.verification}`}><td>{p.method}/{p.verification}</td><td>{money(p.amount_cents)}</td><td>{money(p.tendered_cents)}</td><td>{money(p.change_cents)}</td></tr>)}</tbody></table></div>
    </>}</ResourceState>
      <h3>Catatan koreksi (append-only)</h3><ResourceState resource={notes} empty={!notes.data?.notes?.length} message="Belum ada catatan koreksi.">{notes.data?.notes?.map(n => <article key={n.note_id}><p style={{ whiteSpace: 'pre-wrap' }}>{n.body}</p><small>{n.author_name} · {time(n.created_at)} · {n.note_id}</small><hr /></article>)}</ResourceState>
      <div className="row no-print"><button disabled={!noteCursor || pending.blocked} onClick={() => setNoteCursor('')}>Catatan halaman pertama</button><button disabled={pending.blocked || notes.loading || !notes.data?.next_cursor} onClick={() => setNoteCursor(notes.data.next_cursor)}>Catatan berikutnya</button><button disabled={summary.loading || !!summary.error} onClick={print}>Cetak halaman ringkasan/catatan</button><button disabled={pending.blocked} onClick={() => setSelected('')}>Tutup</button></div>
      {canNote && <form className="no-print" onSubmit={addNote}><fieldset disabled={pending.blocked || summary.loading || !!summary.error}><label>Catatan baru (tidak dapat diubah/dihapus)<textarea value={body} onChange={e => setBody(e.target.value)} maxLength={1000} required rows={4} /></label><p className="muted">Jangan isi kata sandi, PIN, data resep, atau kontak pelanggan. Untuk koreksi uang gunakan dokumen keuangan yang sesuai, bukan catatan ini.</p><button className="primary">Tambah catatan koreksi</button></fieldset></form>}
    </section>}
  </>;
}
