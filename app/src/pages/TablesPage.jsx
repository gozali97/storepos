import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useRestaurantFeed } from '../hooks/useRestaurantFeed.js';
import { usePendingMutation } from '../hooks/usePendingMutation.js';
import { newIdempotencyKey } from '../api/client.js';
import { canClean, filterTables, groupByArea, tableStatusLabel, tableStatusTone, tableSummary } from '../tables.js';
import { PageHeading, Notice } from '../components/UI.jsx';
import { Icon } from '../components/Icon.jsx';

export default function TablesPage() {
  const { businessId, user } = useSession();
  const { branch } = useWorkspace();
  return <TablesWorkspace key={`${businessId}.${user?.id}.${branch?.id}`} />;
}

function TablesWorkspace() {
  const { client, business, businessId, user } = useSession();
  const { branch } = useWorkspace();
  const tableFeed = useRestaurantFeed(branch ? `/restaurant/tables?branch_id=${branch.id}` : null, branch?.id, 'tables');
  const tables = tableFeed.data?.tables ?? null;
  const error = tableFeed.error?.message;
  const load = tableFeed.reload;
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState('');
  const pending = usePendingMutation(`storepos.pendingTableOrder.${businessId}.${user?.id}.${branch?.id}`);
  const [filters, setFilters] = useState({ area: '', status: '', query: '' });
  const orderStatus = useRestaurantFeed(branch ? `/restaurant/order-status?branch_id=${branch.id}` : null, branch?.id);
  const statusByTable = new Map((orderStatus.data?.orders ?? []).filter(order => order.table_id).map(order => [order.table_id, order]));
  const navigate = useNavigate();


  async function clean(table) {
    if (busy || pending.blocked || tableFeed.error || tableFeed.stream === 'offline') return;
    setBusy(table.table_id);
    setMessage('');
    try {
      await client.post(`/restaurant/tables/${table.table_id}/clean`, undefined, { version: table.version });
      setMessage(`Meja ${table.name} ditandai bersih.`);
    } catch (err) {
      // 409: meja berubah di perangkat lain; muat ulang agar versi terbaru dipakai.
      setMessage(err.status === 409 ? 'Meja baru saja berubah. Tampilan dimuat ulang, coba lagi.' : err.message);
    } finally {
      setBusy('');
      await load();
    }
  }

  // Membuka pesanan di meja kosong, atau melanjutkan pesanan terbuka (server mengembalikan pesanan yang sama).
  async function openOrder(table) {
    if (busy || pending.blocked || tableFeed.error || tableFeed.stream === 'offline') return;
    setBusy(table.table_id);
    setMessage('');
    try {
      const result = await pending.send({ path: '/restaurant/orders', body: { branch_id: branch.id, table_id: table.table_id, order_type: 'DINE_IN' }, key: newIdempotencyKey() });
      navigate(`/pesanan/${result.order_id}`);
    } catch (err) {
      setMessage(err.message);
      setBusy('');
    }
  }

  async function retryOrder() {
    if (busy) return;
    setBusy('retry'); setMessage('');
    try { const result = await pending.send(pending.pending); navigate(`/pesanan/${result.order_id}`); }
    catch (error) { setMessage(error.message); }
    finally { setBusy(''); }
  }

  const areas = groupByArea(tables ?? []);
  const groups = groupByArea(filterTables(tables ?? [], filters));
  const summary = tableSummary(tables ?? []);
  const totalTables = Object.values(summary).reduce((sum, count) => sum + count, 0);

  return <>
    <PageHeading title="Meja" description="Temukan meja kosong, lanjutkan pesanan, dan pantau kesiapan area.">
      {['owner', 'manager'].includes(business?.role) && <Link className="button" to="/meja/kelola"><Icon name="settings" size={18} />Kelola meja</Link>}
    </PageHeading>
    {error && <div className="alert warn" role="alert">Data meja belum dapat dimuat: {error} <button onClick={load}>Muat ulang</button></div>}
    {tableFeed.stream === 'offline' && <p role="alert">Perangkat offline. Status meja mungkin tertinggal.</p>}
    <Notice>{message || pending.error}</Notice>
    {pending.pending && <div className="alert warn" role="status">Status buka meja belum pasti.<button disabled={!!busy || pending.sending} onClick={retryOrder}>Periksa order dengan key asli</button></div>}
    {!branch ? <div className="empty-state">Pilih cabang untuk melihat meja.</div>
      : tables === null && !error ? <div className="empty-state" role="status"><span className="loading-ring" />Memuat meja…</div>
      : !areas.length ? <div className="empty-state panel"><Icon name="store" size={32} /><h3>Belum ada meja</h3><p>Buat area dan meja di pengaturan restoran untuk mulai menerima pesanan.</p></div>
      : <>
        <div className="tables-summary" role="group" aria-label="Filter status meja">
          {[['', 'Semua meja', totalTables, 'store'], ['AVAILABLE', 'Kosong', summary.AVAILABLE, 'check'], ['OCCUPIED', 'Terisi', summary.OCCUPIED, 'receipt'], ['NEEDS_CLEANING', 'Perlu dibersihkan', summary.NEEDS_CLEANING, 'refresh']].map(([status, label, count, icon]) => <button key={status} type="button" aria-pressed={filters.status === status} onClick={() => setFilters(value => ({ ...value, status }))}><span className="tables-summary-icon"><Icon name={icon} /></span><span>{label}<b>{count}</b></span></button>)}
        </div>
        <div className="tables-toolbar"><div><h2>{branch.name}</h2><p className="muted small">{totalTables} meja · {areas.length} area · Status diperbarui setiap 5 detik</p></div><div className="tables-filter-fields"><label>Cari meja<input value={filters.query} onChange={event => setFilters(value => ({ ...value, query: event.target.value }))} placeholder="Nama meja atau area" /></label><label>Area<select value={filters.area} onChange={event => setFilters(value => ({ ...value, area: event.target.value }))}><option value="">Semua area</option>{areas.map(area => <option key={area.id} value={area.id}>{area.name}</option>)}</select></label></div></div>
        {orderStatus.error && <p role="alert">Status siap diantar belum dapat diperbarui: {orderStatus.error.message}</p>}
        {orderStatus.data?.truncated && <p role="status">Status pesanan dibatasi server; sebagian indikator siap diantar mungkin belum tampil.</p>}
        {!groups.length && <div className="panel empty-state"><Icon name="search" size={30} /><h3>Tidak ada meja yang cocok</h3><p>Ubah pencarian, area, atau status untuk melihat meja lainnya.</p><button onClick={() => setFilters({ area: '', status: '', query: '' })}>Tampilkan semua meja</button></div>}
        <div className="tables-areas">{groups.map(area => <section key={area.id} className="tables-area" aria-label={`Area ${area.name}`}>
          <div className="tables-area-heading"><h2>{area.name}</h2><span>{area.tables.length} meja ditampilkan</span></div>
          <div className="table-grid">{area.tables.map(table => { const order = statusByTable.get(table.table_id); return <article key={table.table_id} className={`table-card ${tableStatusTone(table.status)}`}>
            <header><span className="table-card-symbol"><Icon name="store" size={24} /></span>
            <span className={`badge ${tableStatusTone(table.status)}`}>{tableStatusLabel(table.status)}</span>
            </header>
            <strong>{table.name}</strong>
            <p className="table-card-capacity"><Icon name="users" size={16} />{table.capacity} kursi<span>· {area.name}</span></p>
            {order?.ready_count > 0 && <span className="badge ok" role="status">{order.ready_count} item siap diantar</span>}
            <p className="table-card-hint">{table.status === 'AVAILABLE' ? 'Siap menerima tamu baru.' : table.status === 'OCCUPIED' ? 'Pesanan sedang berlangsung.' : 'Bersihkan meja sebelum menerima tamu.'}</p>
            {table.status !== 'NEEDS_CLEANING' && <button className={table.status === 'OCCUPIED' ? 'primary' : ''} disabled={!!busy || pending.blocked || !!tableFeed.error || tableFeed.stream === 'offline'} onClick={() => openOrder(table)}>{busy === table.table_id ? 'Membuka…' : table.status === 'OCCUPIED' ? 'Lihat pesanan' : 'Buka pesanan'}<Icon name="arrow" size={17} /></button>}
            {canClean(table) && <button disabled={!!busy || pending.blocked || !!tableFeed.error || tableFeed.stream === 'offline'} onClick={() => clean(table)}><Icon name="check" size={17} />{busy === table.table_id ? 'Menyimpan…' : 'Tandai bersih'}</button>}
          </article>; })}</div>
        </section>)}</div>
      </>}
  </>;
}
