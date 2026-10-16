import { useEffect, useState } from 'react';
import { useRestaurantFeed } from '../hooks/useRestaurantFeed.js';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { formatQty } from '../money.js';
import { activeItems, ageTone, isBackward, kitchenOverview, modifierText, nextAction, previousStatus, statusLabel, ticketAgeMinutes, ticketReady } from '../kds.js';
import { PageHeading, Notice } from '../components/UI.jsx';
import { KitchenTicketPrint } from '../components/KitchenTicketPrint.jsx';
import { Icon } from '../components/Icon.jsx';

export default function KitchenPage() {
  const { businessId, user } = useSession();
  const { branch } = useWorkspace();
  return <KitchenWorkspace key={`${businessId}.${user?.id}.${branch?.id}`} />;
}

function KitchenWorkspace() {
  const { client, business } = useSession();
  const { branch } = useWorkspace();
  const role = business?.role;
  const perms = {
    canPrepare: ['owner', 'manager', 'kitchen'].includes(role),
    canServe: ['owner', 'manager', 'cashier'].includes(role),
  };
  const stations = useResource(branch ? `/restaurant/stations?branch_id=${branch.id}` : null);
  const list = (stations.data?.stations ?? []).filter(s => !s.archived);
  const [stationId, setStationId] = useState('');
  const [filter, setFilter] = useState('all');
  const station = list.find(s => s.station_id === stationId) ?? list[0];
  const resource = useRestaurantFeed(branch && station ? `/restaurant/kitchen/tickets?branch_id=${branch.id}&station_id=${station.station_id}` : null, branch?.id, 'tickets');
  const feed = { tickets: resource.data?.tickets ?? [], serverTime: resource.data?.server_time, fetchedAt: resource.receivedAt };
  const connected = !resource.error && resource.stream !== 'offline';
  const [message, setMessage] = useState('');
  const [busyLine, setBusyLine] = useState('');
  const [printTicket, setPrintTicket] = useState(null);
  const [, setTick] = useState(0);
  const load = resource.reload;
  const [backward, setBackward] = useState(null);

  // Menyegarkan umur tiket tiap 30 detik tanpa memanggil server.
  useEffect(() => {
    const timer = setInterval(() => setTick(n => n + 1), 30000);
    return () => clearInterval(timer);
  }, []);

  async function previewTicket(id) {
    setMessage('');
    try { setPrintTicket(await client.get(`/restaurant/kitchen/tickets/${id}`)); }
    catch (error) { setMessage(error.message); }
  }

  async function move(ticket, item, status, suppliedReason) {
    // Mundur dinilai dari urutan status asal -> tujuan, bukan dari nama status tujuan.
    if (busyLine || !connected) return;
    let reason = suppliedReason;
    if (isBackward(item.status, status) && !reason?.trim()) {
      setBackward({ ticket, item, status });
      return;
    }
    if (reason && [...reason.trim()].length > 200) { setMessage('Alasan maksimal 200 karakter.'); return; }
    setBusyLine(item.line_id);
    setMessage('');
    try {
      await client.patch(`/restaurant/orders/${ticket.order_id}/lines/${item.line_id}/kitchen-status`,
        { status, ...(reason ? { reason: reason.trim() } : {}) }, { version: ticket.order_version });
      setBackward(null);
      await load();
    } catch (error) {
      // 409: pesanan berubah di perangkat lain; muat ulang agar tombol memakai versi terbaru.
      setMessage(error.status === 409 ? 'Pesanan baru saja berubah. Tampilan dimuat ulang, coba lagi.' : error.message);
      await load();
    } finally {
      setBusyLine('');
    }
  }

  const currentFeed = !!resource.data;
  const tickets = currentFeed ? feed.tickets : [];
  const { counts: statusCounts, tickets: visibleTickets } = kitchenOverview(tickets, filter);
  const orderTypes = { DINE_IN: 'Makan di tempat', TAKEAWAY: 'Bawa pulang', DELIVERY_MANUAL: 'Delivery manual' };

  return <>
    <PageHeading title="Layar dapur" description="Pantau pesanan masuk, proses masak, dan menu yang siap diantar.">
      {list.length > 0 && <label className="field">Stasiun dapur
        <select value={station?.station_id ?? ''} onChange={event => setStationId(event.target.value)}>
          {list.map(s => <option key={s.station_id} value={s.station_id}>{s.name}</option>)}
        </select>
      </label>}
    </PageHeading>
    {!connected && <div className="alert warn" role="alert">Koneksi terputus. Tampilan mungkin tertinggal; mencoba lagi otomatis.</div>}
    <Notice>{message || resource.error?.message}</Notice>
    {resource.stream === 'polling' && <div className="alert warn" role="status">Stream terputus/tidak tersedia; polling 5 detik tetap berjalan.</div>}
    <button disabled={resource.loading} onClick={load}>Muat ulang tiket</button>
    {backward && <section className="panel form-panel"><h2>Mundurkan status {backward.item.snapshot?.product_name}</h2><form onSubmit={event => { event.preventDefault(); move(backward.ticket, backward.item, backward.status, new FormData(event.currentTarget).get('reason')); }}><fieldset disabled={!!busyLine || !connected}><label>Alasan wajib<input name="reason" required maxLength={200} autoFocus /></label><button className="primary">Simpan perubahan</button><button type="button" onClick={() => setBackward(null)}>Batal</button></fieldset></form></section>}
    <Notice>{stations.error?.message}</Notice>
    {branch && station && <>
      <div className="kds-overview">
        <div className="kds-station-summary"><span className="soft-icon"><Icon name="receipt" size={24} /></span><div><p className="muted small">{branch.name}</p><h2>{station.name}</h2><p>{currentFeed ? `${tickets.length} tiket aktif` : 'Memuat tiket…'}</p></div></div>
        <div className="kds-sync"><Icon name="refresh" size={16} /><span>{!connected ? 'Menunggu koneksi' : currentFeed ? `Pembaruan terakhir ${new Date(resource.receivedAt).toLocaleTimeString('id-ID')} · polling cadangan 5 detik` : 'Menghubungkan ke dapur…'}</span></div>
      </div>
      <div className="kds-filters" role="group" aria-label="Filter status tiket">
        {[['all', 'Semua tiket', tickets.length], ['SENT', 'Baru', statusCounts.SENT], ['IN_PROGRESS', 'Dimasak', statusCounts.IN_PROGRESS], ['READY', 'Siap diantar', statusCounts.READY]].map(([value, label, count]) => <button type="button" key={value} aria-pressed={filter === value} onClick={() => setFilter(value)}><span>{label}</span><b>{currentFeed ? count : '…'}</b><small>{value === 'all' ? 'tiket' : 'item menu'}</small></button>)}
      </div>
    </>}
    {!branch ? <div className="empty-state">Pilih cabang untuk membuka layar dapur.</div>
      : stations.loading ? <div className="empty-state" role="status">Memuat stasiun dapur…</div>
      : !station ? <div className="empty-state">Belum ada stasiun dapur. Buat stasiun di pengaturan restoran.</div>
      : !currentFeed ? <div className="empty-state" role="status">Memuat tiket stasiun {station.name}…</div>
      : !visibleTickets.length ? <div className="empty-state panel" role="status"><Icon name="receipt" size={32} /><h3>{filter === 'all' ? 'Belum ada pesanan masuk' : 'Tidak ada tiket pada status ini'}</h3><p>{filter === 'all' ? 'Pesanan yang dikirim ke stasiun ini akan muncul otomatis.' : 'Pilih Semua tiket untuk melihat pesanan pada status lainnya.'}</p></div>
      : <div className="kds-grid">{visibleTickets.map(ticket => {
        const age = ticketAgeMinutes(ticket.created_at, feed.serverTime, feed.fetchedAt);
        const ready = ticketReady(ticket);
        return <article key={ticket.ticket_id} className={`kds-ticket ${ageTone(age)} ${ready ? 'ready' : ''}`} aria-label={`Tiket ${ticket.table_name ?? ticket.order_type}`}>
          <header>
            <div className="kds-ticket-identity"><span>{orderTypes[ticket.order_type] ?? ticket.order_type}</span><strong>{ticket.table_name || 'Pesanan tanpa meja'}</strong><small>Tiket #{ticket.ticket_id.slice(-6).toUpperCase()}</small></div>
            <button className="icon-button no-print" title="Cetak tiket" aria-label={`Cetak tiket ${ticket.table_name || ticket.ticket_id.slice(-6)}`} onClick={() => previewTicket(ticket.ticket_id)}><Icon name="receipt" size={19} /></button>
          </header>
          <div className="kds-ticket-meta"><span className="kds-age"><Icon name="clock" size={16} />{age} menit</span><span>{age >= 20 ? 'Perlu diprioritaskan' : age >= 10 ? 'Periksa waktu tunggu' : 'Sejak tiket masuk'}</span></div>
          {ready && <p className="kds-ready">Siap diantar</p>}
          <ul>{activeItems(ticket).map(item => {
            const snap = item.snapshot ?? {};
            const action = nextAction(item.status, perms);
            const back = (item.status === 'SERVED' ? perms.canServe : perms.canPrepare) ? previousStatus(item.status) : null;
            const mods = modifierText(snap.modifiers);
            return <li key={item.line_id} className={item.status === 'CANCELLED' ? 'cancelled' : ''}>
              <div className="kds-item-info">
                <strong>{formatQty(BigInt(snap.qty_milli ?? 0))} × {snap.product_name}</strong>
                {snap.variant_name ? <small> {snap.variant_name}</small> : null}
                {mods && <small className="kds-note">{mods}</small>}
                {snap.note && <small className="kds-note">Catatan: {snap.note}</small>}
                {item.status === 'CANCELLED'
                  ? <small className="kds-note" role="alert">DIBATALKAN{item.cancel_reason ? `: ${item.cancel_reason}` : ''}</small>
                  : <span className={`kds-item-status status-${item.status.toLowerCase()}`}>{statusLabel(item.status)}</span>}
              </div>
              {item.status !== 'CANCELLED' && <div className="kds-item-actions">
                {back && <button disabled={!!busyLine || !connected} aria-label={`Mundurkan ${snap.product_name}`} onClick={() => move(ticket, item, back)}>↩</button>}
                {action && <button className="primary kds-action" disabled={!!busyLine || !connected} onClick={() => move(ticket, item, action.next)}>{busyLine === item.line_id ? 'Memproses…' : action.label}<Icon name="arrow" size={18} /></button>}
              </div>}
            </li>;
          })}</ul>
          <footer className="kds-ticket-footer">{activeItems(ticket).filter(item => item.status !== 'CANCELLED').length} item menu · {station.name}</footer>
        </article>;
      })}</div>}
    <KitchenTicketPrint ticket={printTicket} onClose={() => setPrintTicket(null)} />
  </>;
}
