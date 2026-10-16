import { useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { localDate } from '../reporting.js';
import { formatIDR, formatQty } from '../money.js';
import { PageHeading, Notice, ResourceState } from '../components/UI.jsx';

export default function RestaurantReportsPage() {
  const { business, businessId, user } = useSession();
  const { branch } = useWorkspace();
  if (business?.business_type !== 'restaurant' || !['owner', 'manager'].includes(business?.role)) return <Notice>Laporan operasional restoran hanya untuk Pemilik/Manajer.</Notice>;
  if (!branch) return <Notice>Pilih cabang aktif untuk melihat laporan.</Notice>;
  return <Reports key={`${businessId}.${user?.id}.${branch.id}`} />;
}
function Reports() {
  const { client } = useSession(); const { branch } = useWorkspace();
  const [tab, setTab] = useState('cancellations');
  const [from, setFrom] = useState(localDate(branch.timezone)); const [to, setTo] = useState(localDate(branch.timezone));
  const [actor, setActor] = useState(''); const [hours, setHours] = useState('4'); const [cursor, setCursor] = useState('');
  const [message, setMessage] = useState(''); const [busy, setBusy] = useState(false); const lock = useRef(false);
  const validDates = /^\d{4}-\d{2}-\d{2}$/.test(from) && /^\d{4}-\d{2}-\d{2}$/.test(to) && from <= to && (Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) <= 366 * 86400000;
  const validActor = !actor.trim() || /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(actor.trim());
  const validHours = /^\d+$/.test(hours) && Number(hours) >= 1 && Number(hours) <= 720;
  const params = new URLSearchParams({ branch_id: branch.id, from, to, ...(actor.trim() ? { user_id: actor.trim() } : {}) });
  const cancellationPath = validDates && validActor ? `/restaurant/reports/cancellations?${params}` : null;
  const cancellations = useResource(tab === 'cancellations' ? cancellationPath : null);
  const aging = useResource(tab === 'aging' && validHours ? `/restaurant/reports/aged-orders?branch_id=${branch.id}&hours=${hours}${cursor ? `&after=${cursor}` : ''}` : null);
  async function download() {
    if (lock.current || !cancellationPath) return;
    lock.current = true; setBusy(true); setMessage(''); let url;
    try {
      const blob = await client.get(`${cancellationPath}&format=csv`, { responseType: 'blob' });
      url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href = url; a.download = `pembatalan-dapur-${from}-${to}.csv`; a.click();
    } catch (error) { setMessage(error.message); }
    finally { if (url) setTimeout(() => URL.revokeObjectURL(url), 1000); lock.current = false; setBusy(false); }
  }
  const time = value => new Date(value).toLocaleString('id-ID', { timeZone: branch.timezone });
  return <>
    <PageHeading title="Laporan operasional restoran" description={`Cabang ${branch.name} · ${branch.timezone}`}><Link to="/pesanan">Daftar pesanan</Link></PageHeading>
    <Notice>{message}</Notice>
    <div className="row" role="group" aria-label="Jenis laporan restoran"><button aria-pressed={tab === 'cancellations'} onClick={() => { setTab('cancellations'); setMessage(''); }}>Pembatalan item</button><button aria-pressed={tab === 'aging'} onClick={() => { setTab('aging'); setMessage(''); }}>Pesanan terlalu lama</button></div>
    {tab === 'cancellations' && <section className="panel"><h2>Pembatalan item dapur</h2><div className="filter-fields"><label>Dari<input type="date" value={from} max={to} onChange={e => setFrom(e.target.value)} /></label><label>Sampai<input type="date" value={to} min={from} onChange={e => setTo(e.target.value)} /></label><label>Pelaku (UUID, opsional)<input value={actor} onChange={e => setActor(e.target.value)} maxLength={36} /></label><button disabled={busy || !cancellationPath || cancellations.loading || !!cancellations.error} onClick={download}>Unduh CSV filter ini</button><button disabled={!cancellationPath || cancellations.loading} onClick={cancellations.reload}>Muat ulang</button></div>
      {(!validDates || !validActor) && <Notice>Periksa periode maksimal 366 hari dan ID pelaku.</Notice>}
      <p className="muted">{cancellations.data?.value_label ?? 'Nilai menu memakai harga jual snapshot, bukan HPP atau kerugian. Potensi waste bukan bukti pemusnahan fisik.'}</p>
      <ResourceState resource={cancellations} empty={!cancellations.data?.rows?.length} message="Tidak ada pembatalan pada filter ini."><div className="table-scroll"><table><thead><tr><th>Waktu lokal</th><th>Pesanan/menu</th><th>Jumlah</th><th>Modifier snapshot</th><th>Status asal</th><th>Potensi waste</th><th>Alasan/pelaku/penyetuju</th><th>Nilai menu</th><th>Dokumen</th></tr></thead><tbody>{cancellations.data?.rows?.map(r => <tr key={r.line_id}><td>{time(r.cancelled_at)}</td><td>{r.order_type} · {r.product_name}{r.variant_name ? ` / ${r.variant_name}` : ''}</td><td>{formatQty(BigInt(r.qty_milli))} {r.base_unit}</td><td>{r.modifiers?.map((m, i) => <div key={m.option_id ?? i}>{m.group_name ? `${m.group_name}: ` : ''}{m.name}</div>)}</td><td>{r.cancelled_from_status}</td><td>{r.potential_waste ? 'Ya — perlu pemeriksaan' : 'Tidak ditandai'}</td><td><p style={{ whiteSpace: 'pre-wrap' }}>{r.reason}</p><small>Pelaku {r.actor_user_id}<br />Penyetuju {r.approved_by_user_id ?? 'Tidak ada'}</small></td><td>{formatIDR(BigInt(r.menu_value_cents))}</td><td><Link to={`/pesanan/${r.order_id}`}>Buka pesanan</Link></td></tr>)}</tbody></table></div></ResourceState>
    </section>}
    {tab === 'aging' && <section className="panel"><h2>Pesanan terbuka terlalu lama</h2><div className="filter-fields"><label>Lebih dari (jam, 1–720)<input type="number" min="1" max="720" step="1" value={hours} onChange={e => { setHours(e.target.value); setCursor(''); }} /></label><button disabled={!validHours || aging.loading} onClick={aging.reload}>Muat ulang umur server</button></div>{!validHours && <Notice>Ambang jam harus bilangan bulat 1–720.</Notice>}
      <p className="muted">Umur dihitung server; laporan ini tidak membatalkan atau menutup pesanan otomatis.</p>
      <ResourceState resource={aging} empty={!aging.data?.orders?.length} message="Tidak ada pesanan melewati ambang ini."><div className="table-scroll"><table><thead><tr><th>Jenis/meja</th><th>Dibuat (zona cabang)</th><th>Umur</th><th>Versi</th><th>Aksi</th></tr></thead><tbody>{aging.data?.orders?.map(o => <tr key={o.order_id}><td>{o.order_type} · {o.table_name ?? 'Tanpa meja'}</td><td>{time(o.created_at)}</td><td>{Math.floor(o.age_seconds / 3600)} jam {Math.floor(o.age_seconds % 3600 / 60)} menit</td><td>{o.version}</td><td><Link to={`/pesanan/${o.order_id}`}>Periksa pesanan</Link></td></tr>)}</tbody></table></div></ResourceState><div className="row"><button disabled={!cursor} onClick={() => setCursor('')}>Halaman pertama</button><button disabled={aging.loading || !aging.data?.next_cursor} onClick={() => setCursor(aging.data.next_cursor)}>Berikutnya</button></div>
    </section>}
  </>;
}
