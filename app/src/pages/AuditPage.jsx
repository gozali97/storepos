import { useEffect, useState } from "react";
import { Link } from 'react-router-dom';
import { useWorkspace } from "../workspace/WorkspaceContext.jsx";
import { useResource } from "../hooks/useResource.js";
import { PageHeading, ResourceState } from "../components/UI.jsx";
import { Icon } from "../components/Icon.jsx";
export default function AuditPage() {
  const {
    branch
  } = useWorkspace();
  const [from, setFrom] = useState(new Date(Date.now() - 7 * 864e5).toISOString().slice(0, 16));
  const [until, setUntil] = useState(new Date().toISOString().slice(0, 16));
  const [cursor, setCursor] = useState("");
  const [action, setAction] = useState("");
  useEffect(() => setCursor(''), [branch?.id]);
  const query = new URLSearchParams({
    from: `${from}:00Z`,
    until: `${until}:00Z`,
    ...(branch ? {
      branch_id: branch.id
    } : {}),
    ...(cursor ? {
      before: cursor
    } : {}),
    ...(action ? {
      action
    } : {})
  });
  const valid = from && until && from < until;
  const resource = useResource(valid ? `/audit-logs?${query}` : null);
  const rows = resource.data?.events ?? [];
  return <>
  <PageHeading title="Log aktivitas" description="Riwayat tindakan dan otorisasi pada bisnis serta cabang aktif."><Link to="/rekonsiliasi">Lihat temuan rekonsiliasi</Link></PageHeading>
  <section className="panel">
    <div className="table-toolbar">
      <div className="filter-fields">
        <label>Dari (UTC)<input type="datetime-local" value={from} max={until} onChange={e => {
              setFrom(e.target.value);
              setCursor("");
            }} /></label>
        <label>Sampai (UTC, eksklusif)<input type="datetime-local" value={until} min={from} onChange={e => {
              setUntil(e.target.value);
              setCursor("");
            }} /></label>
        <label>Tindakan<select value={action} onChange={e => {
              setAction(e.target.value);
              setCursor("");
            }}>
            <option value="">Semua tindakan</option>
            <option value="product.created">Produk dibuat</option>
            <option value="variant.price_changed">Harga diubah</option>
            <option value="supplier.saved">Supplier disimpan</option>
            <option value="expense.created">Pengeluaran dibuat</option>
            <option value="report.exported">Laporan diekspor</option>
          </select></label>
      </div>
      <button aria-label="Muat ulang log aktivitas" onClick={resource.reload} className="icon-button"><Icon name="refresh" /></button>
    </div>
    {!valid ? <div className="alert error" role="alert">Tanggal awal harus sebelum tanggal akhir.</div> : <ResourceState resource={resource} empty={!rows.length} message="Tidak ada aktivitas dalam filter ini. Pilih periode atau tindakan lain."><div className="table-scroll"><table>
          <thead><tr>
              <th>Waktu (UTC)</th>
              <th>Tindakan</th>
              <th>Entitas</th>
              <th>Pelaku</th>
              <th>Otorisasi</th>
              <th>Detail</th>
            </tr></thead>
          <tbody>{rows.map(row => <tr key={row.id}>
              <td>{new Intl.DateTimeFormat("id-ID", {
                    dateStyle: "short",
                    timeStyle: "short",
                    timeZone: "UTC"
                  }).format(new Date(row.occurred_at))}</td>
              <td><strong>{row.action}</strong></td>
              <td>{row.entity_type}</td>
              <td>{row.actor_user_id ?? "Sistem"}</td>
              <td>{row.approved_by_user_id ?? "—"}</td>
              <td><details>
                  <summary>Lihat detail</summary>
                  <p>{row.reason}</p>
                  <pre>{JSON.stringify({
                        sebelum: row.before,
                        sesudah: row.after
                      }, null, 2)}</pre>
                </details></td>
            </tr>)}</tbody>
        </table></div></ResourceState>}
    <div className="table-footer">
      <span>{rows.length} aktivitas</span>
      <div className="row">
        {cursor && <button onClick={() => setCursor("")}>Terbaru</button>}
        <button disabled={!resource.data?.next_cursor} onClick={() => setCursor(resource.data.next_cursor)}>Lebih lama</button>
      </div>
    </div>
  </section>
</>;
}
