import { useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { usePendingMutation } from '../hooks/usePendingMutation.js';
import { newIdempotencyKey } from '../api/client.js';
import { localDate } from '../reporting.js';
import { formatIDR } from '../money.js';
import { restaurantHeaderBody } from '../restaurantHeaders.js';
import { PageHeading, Notice, ResourceState } from '../components/UI.jsx';

export default function RestaurantOrdersPage() {
  const { business, businessId, user } = useSession();
  const { branch } = useWorkspace();
  if (business?.business_type !== 'restaurant' || !['owner', 'manager', 'cashier'].includes(business?.role)) return <Notice>Daftar pesanan hanya untuk petugas restoran berizin.</Notice>;
  return <Orders key={`${businessId}.${user?.id}.${branch?.id}`} />;
}
function Orders() {
  const { client, businessId, user } = useSession(); const { branch } = useWorkspace(); const navigate = useNavigate();
  const storageKey = `storepos.orderCreateUncertain.${businessId}.${user?.id}.${branch?.id}`;
  const pending = usePendingMutation(`storepos.pendingOrderCreate.${businessId}.${user?.id}.${branch?.id}`);
  const [from, setFrom] = useState(localDate(branch?.timezone)); const [to, setTo] = useState(localDate(branch?.timezone));
  const [status, setStatus] = useState('open'); const [type, setType] = useState(''); const [cursor, setCursor] = useState('');
  const [createType, setCreateType] = useState('TAKEAWAY'); const [show, setShow] = useState(false); const [busy, setBusy] = useState(false); const [message, setMessage] = useState(''); const [uncertain, setUncertain] = useState(() => { try { return sessionStorage.getItem(storageKey) === 'pending'; } catch { return true; } });
  const sending = useRef(false);
  const query = new URLSearchParams({ branch_id: branch?.id ?? '', status, ...(type ? { order_type: type } : {}), ...(status !== 'open' ? { from, to } : {}), ...(cursor ? { after: cursor } : {}) });
  const list = useResource(branch && (status === 'open' || from && to && from <= to) ? `/restaurant/orders?${query}` : null);
  async function create(event) {
    event.preventDefault(); if (sending.current || uncertain || pending.blocked || !branch) return;
    const values = Object.fromEntries(new FormData(event.currentTarget));
    sending.current = true; setBusy(true); setMessage('');
    try { const body = { branch_id: branch.id, order_type: createType, ...restaurantHeaderBody(createType, values) }; const result = await pending.send({ path: '/restaurant/orders', body, key: newIdempotencyKey() }); navigate(`/pesanan/${result.order_id}`); }
    catch (error) {
      setMessage(error.message); list.reload();
    } finally { setBusy(false); sending.current = false; }
  }
  async function retry() {
    if (busy) return;
    setBusy(true); setMessage('');
    try { const result = await pending.send(pending.pending); navigate(`/pesanan/${result.order_id}`); }
    catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }
  return <>
    <PageHeading title="Pesanan restoran" description="Pesanan terbuka lintas terminal pada cabang aktif; pesanan baru belum terikat shift pembayaran."><Link to="/meja">Grid meja</Link><Link to="/restoran/laporan">Pembatalan & pesanan lama (sesuai izin)</Link><button disabled={!branch || busy || uncertain || pending.blocked} onClick={() => setShow(v => !v)}>{show ? 'Tutup form' : 'Takeaway / delivery manual'}</button></PageHeading>
    <Notice>{message || pending.error}</Notice>
    {pending.pending && <div className="alert warn" role="status">Status order belum pasti. Retry memakai payload dan key asli, bukan membuat order baru.<button disabled={busy || pending.sending} onClick={retry}>Periksa & coba lagi dengan key asli</button></div>}
    {uncertain && <div className="alert warn" role="alert">Status pembuatan pesanan belum pasti. Jangan membuat ulang sebelum memeriksa daftar pesanan terbuka (termasuk setelah refresh).<button onClick={() => { setStatus('open'); setType(''); setCursor(''); list.reload(); }}>Periksa daftar</button><button onClick={() => { if (window.confirm('Sudah memeriksa server dan memastikan pesanan belum dibuat?')) { try { sessionStorage.removeItem(storageKey); setUncertain(false); } catch (error) { setMessage(error.message); } } }}>Saya sudah memastikan belum dibuat</button></div>}
    {show && <section className="panel form-panel"><h2>Pesanan tanpa meja</h2><form onSubmit={create}><fieldset disabled={busy || uncertain || pending.blocked}>
      <label>Jenis<select value={createType} onChange={e => setCreateType(e.target.value)}><option value="TAKEAWAY">Takeaway</option><option value="DELIVERY_MANUAL">Delivery manual</option></select></label>
      <label>Nama pelanggan/penerima (opsional)<input name="contact_name" maxLength={100} /></label><label>Nomor antrean (opsional)<input name="queue_label" maxLength={40} /></label>
      {createType === 'DELIVERY_MANUAL' && <><label>Kanal (nama bebas)<input name="channel" maxLength={100} /></label><label>Telepon penerima (opsional)<input name="contact_phone" maxLength={32} /></label><label>Ongkir (Rp)<input name="fee" type="number" min="0" max="100000000" step="0.01" defaultValue="0" /></label></>}
      <label>Catatan<input name="note" maxLength={200} /></label><button className="primary">Buat dan buka pesanan</button>
    </fieldset></form></section>}
    <section className="panel"><div className="filter-fields"><label>Status<select value={status} onChange={e => { setStatus(e.target.value); setCursor(''); }}><option value="open">Terbuka</option><option value="completed">Selesai</option><option value="cancelled">Dibatalkan</option><option value="all">Semua</option></select></label><label>Jenis<select value={type} onChange={e => { setType(e.target.value); setCursor(''); }}><option value="">Semua</option><option value="DINE_IN">Dine-in</option><option value="TAKEAWAY">Takeaway</option><option value="DELIVERY_MANUAL">Delivery manual</option></select></label>
      {status !== 'open' && <><label>Dari<input type="date" value={from} max={to} onChange={e => { setFrom(e.target.value); setCursor(''); }} /></label><label>Sampai<input type="date" value={to} min={from} onChange={e => { setTo(e.target.value); setCursor(''); }} /></label></>}
      <button onClick={list.reload}>Muat ulang</button></div>
      {!branch ? <p>Pilih cabang.</p> : <ResourceState resource={list} empty={!list.data?.orders?.length}><div className="table-scroll"><table><thead><tr><th>Pesanan</th><th>Jenis</th><th>Status/pembayaran</th><th>Antrean/kanal</th><th>Ongkir</th><th>Dibuat</th><th>Aksi</th></tr></thead><tbody>{list.data?.orders?.map(o => <tr key={o.order_id}><td>{o.order_id}</td><td>{o.order_type}</td><td>{o.status}/{o.payment_status}</td><td>{o.queue_label || o.channel || '—'}</td><td>{formatIDR(BigInt(o.delivery_fee_cents))}</td><td>{new Date(o.created_at).toLocaleString('id-ID', { timeZone: branch.timezone })}</td><td><Link to={`/pesanan/${o.order_id}`}>Buka</Link></td></tr>)}</tbody></table></div></ResourceState>}
      <div className="row"><button disabled={!cursor} onClick={() => setCursor('')}>Halaman pertama</button><button disabled={list.loading || !list.data?.next_cursor} onClick={() => setCursor(list.data.next_cursor)}>Berikutnya</button></div>
    </section>
  </>;
}
