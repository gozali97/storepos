import { useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { formatQty } from '../money.js';
import { availabilityLabel, availabilityRequest } from '../orders.js';
import { PageHeading, Notice, ResourceState } from '../components/UI.jsx';

export default function MenuAvailabilityPage() {
  const { client, business } = useSession();
  const { branch } = useWorkspace();
  const [cursor, setCursor] = useState('');
  const [busy, setBusy] = useState('');
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');
  const canEdit = ['owner', 'manager'].includes(business?.role); // kasir hanya melihat (table.manage ReadOnly)
  const params = new URLSearchParams({ branch_id: branch?.id ?? '', ...(cursor ? { after: cursor } : {}) });
  const resource = useResource(branch ? `/restaurant/menu/availability?${params}` : null);
  const items = resource.data?.items ?? [];

  async function toggle(item) {
    if (busy) return;
    setBusy(item.variant_id); setMessage(''); setSuccess('');
    try {
      const { body, version } = availabilityRequest(branch.id, item, !item.sold_out);
      await client.put(`/restaurant/menu/${item.variant_id}/availability`, body, { version });
      setSuccess(`${item.product_name} ${item.sold_out ? 'tersedia kembali' : 'ditandai habis'}.`);
    } catch (error) {
      // 409: ditandai perangkat lain; muat ulang agar versi terbaru dipakai.
      setMessage(error.status === 409 ? 'Status menu baru saja berubah. Tampilan dimuat ulang, coba lagi.' : error.message);
    } finally {
      setBusy('');
      resource.reload();
    }
  }

  return <>
    <PageHeading title="Menu habis" description="Tandai menu yang habis. Menu bertanda habis tidak bisa ditambahkan ke pesanan baru." />
    <Notice>{message}</Notice>
    <Notice kind="success">{success}</Notice>
    <section className="panel">
      {!branch ? <div className="empty-state">Pilih cabang untuk mengatur menu.</div>
        : <ResourceState resource={resource} empty={!items.length} message="Belum ada menu aktif.">
          <div className="table-scroll"><table>
            <thead><tr><th>Menu</th><th>Status</th><th className="num">Stok tersedia</th>{canEdit && <th>Aksi</th>}</tr></thead>
            <tbody>{items.map(item => <tr key={item.variant_id}>
              <td><strong>{item.product_name}</strong>{item.variant_name ? <small className="table-subtext">{item.variant_name}</small> : null}</td>
              <td><span className={`badge ${item.sold_out || (item.track_stock && BigInt(item.available_milli) <= 0n) ? 'danger' : 'ok'}`}>{availabilityLabel(item)}</span></td>
              <td className="num">{item.track_stock ? formatQty(BigInt(item.available_milli)) : '—'}</td>
              {canEdit && <td><button disabled={!!busy} aria-pressed={item.sold_out} onClick={() => toggle(item)}>{item.sold_out ? 'Tersedia kembali' : 'Tandai habis'}</button></td>}
            </tr>)}</tbody>
          </table></div>
        </ResourceState>}
      <div className="table-footer">
        <span>{items.length} menu</span>
        <div className="row">
          {cursor && <button onClick={() => setCursor('')}>Halaman pertama</button>}
          <button disabled={!resource.data?.next_cursor} onClick={() => setCursor(resource.data.next_cursor)}>Berikutnya</button>
        </div>
      </div>
    </section>
  </>;
}
