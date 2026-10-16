import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useSession } from '../auth/SessionContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { PageHeading, Notice, ResourceState } from '../components/UI.jsx';

const LABELS = { pending: 'Menunggu worker', processing: 'Diproses', completed: 'Siap diunduh', failed: 'Gagal', expired: 'Kedaluwarsa' };
export default function ExportHistoryPage() {
  const { businessId, user } = useSession();
  return <History key={`${businessId}.${user?.id}`} />;
}
function History() {
  const { client } = useSession();
  const [cursor, setCursor] = useState('');
  const [busy, setBusy] = useState('');
  const [message, setMessage] = useState('');
  const [selected, setSelected] = useState('');
  const resource = useResource(`/report-exports${cursor ? `?after=${cursor}` : ''}`);
  const detail = useResource(selected ? `/report-exports/${selected}` : null);
  async function download(item) {
    if (busy) return;
    setBusy(item.export_id); setMessage('');
    let url;
    try {
      const blob = await client.get(`/report-exports/${item.export_id}?download=true`, { responseType: 'blob' });
      url = URL.createObjectURL(blob);
      const a = document.createElement('a'); a.href = url; a.download = `${item.report_kind}-${item.export_id}.csv`; a.click();
    } catch (error) { setMessage(error.message); resource.reload(); }
    finally { if (url) setTimeout(() => URL.revokeObjectURL(url), 1000); setBusy(''); }
  }
  return <>
    <PageHeading title="Riwayat ekspor" description="Hanya ekspor yang Anda minta pada bisnis aktif dan cabang yang masih diizinkan."><Link to="/laporan">Buat ekspor di laporan</Link></PageHeading>
    <Notice>{message}</Notice>
    <section className="panel"><button onClick={resource.reload} disabled={!!busy || resource.loading}>Muat ulang antrean</button><p className="muted">Worker harus berjalan untuk memproses antrean. Hasil kedaluwarsa tidak bisa diunduh; izin dicek ulang oleh server ketika mengunduh.</p>
      <ResourceState resource={resource} empty={!resource.data?.exports?.length}><div className="table-scroll"><table><thead><tr><th>Jenis/ID</th><th>Status</th><th>Baris</th><th>Dibuat</th><th>Kedaluwarsa</th><th>Aksi</th></tr></thead><tbody>{resource.data?.exports?.map(item => { const expired = new Date(item.expires_at).getTime() <= Date.now(); return <tr key={item.export_id}><td>{item.report_kind}<small className="table-subtext">{item.export_id}</small></td><td>{expired ? LABELS.expired : LABELS[item.status] ?? item.status}</td><td>{item.row_count == null ? 'Belum tersedia' : item.row_count}</td><td>{new Date(item.created_at).toLocaleString('id-ID')}</td><td>{new Date(item.expires_at).toLocaleString('id-ID')}</td><td><div className="row"><button disabled={!!busy} onClick={() => { setSelected(item.export_id); if (selected === item.export_id) detail.reload(); }}>Detail/status</button><button disabled={!!busy || expired || item.status !== 'completed'} onClick={() => download(item)}>{busy === item.export_id ? 'Mengunduh…' : 'Unduh CSV'}</button></div></td></tr>; })}</tbody></table></div></ResourceState>
      <div className="row"><button disabled={!!busy || !cursor} onClick={() => setCursor('')}>Halaman pertama</button><button disabled={!!busy || resource.loading || !resource.data?.next_cursor} onClick={() => setCursor(resource.data.next_cursor)}>Berikutnya</button></div>
    </section>
    {selected && <section className="panel"><h2>Status ekspor {selected}</h2><ResourceState resource={detail}>{detail.data && <><p>{LABELS[detail.data.status] ?? detail.data.status}</p><p>Jumlah baris: {detail.data.row_count ?? 'Belum tersedia'}</p>{detail.data.error_code && <Notice>Kode kegagalan: {detail.data.error_code}</Notice>}<p>Berlaku sampai {new Date(detail.data.expires_at).toLocaleString('id-ID')}</p></>}</ResourceState><button onClick={detail.reload}>Periksa status server</button><button onClick={() => setSelected('')}>Tutup</button></section>}
  </>;
}
