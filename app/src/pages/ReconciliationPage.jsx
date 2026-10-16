import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useSession } from '../auth/SessionContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { PageHeading, Notice, ResourceState } from '../components/UI.jsx';

export default function ReconciliationPage() {
  const { business, businessId, user } = useSession();
  if (!['owner', 'manager'].includes(business?.role)) return <Notice>Temuan rekonsiliasi hanya untuk Pemilik/Manajer berizin audit.</Notice>;
  return <Findings key={`${businessId}.${user?.id}`} />;
}
function Findings() {
  const [date, setDate] = useState(new Date().toISOString().slice(0,10));
  const [kind, setKind] = useState('');
  const validDate = /^\d{4}-\d{2}-\d{2}$/.test(date);
  const resource = useResource(validDate ? `/reconciliation/findings?date=${date}` : null);
  const data = resource.data;
  const rows = (data?.findings ?? []).filter(row => !kind || row.kind === kind);
  const decimalDifference = (expected, actual) => {
    // Jangan melewatkan numeric PostgreSQL melalui Number yang dapat kehilangan presisi.
    if (/^-?\d+$/.test(expected) && /^-?\d+$/.test(actual)) return (BigInt(actual)-BigInt(expected)).toString();
    return 'Lihat nilai desimal asli';
  };
  return <>
    <PageHeading title="Temuan rekonsiliasi" description="Hasil worker harian; tanggal pelaksanaan memakai UTC, bukan tanggal lokal cabang."><Link to="/audit">Audit aktivitas</Link></PageHeading>
    <section className="panel"><div className="filter-fields"><label>Tanggal run (UTC)<input type="date" value={date} onChange={e => setDate(e.target.value)} /></label><label>Jenis<select value={kind} onChange={e => setKind(e.target.value)}><option value="">Semua</option><option value="stock">Stok</option><option value="payment">Pembayaran</option><option value="reservation">Reservasi</option></select></label><button disabled={!validDate || resource.loading} onClick={resource.reload}>Periksa hasil terbaru</button></div>
      <ResourceState resource={resource}>{data && <>
        {!data.run_started_at ? <div className="alert warn" role="status">Belum ada pelaksanaan worker pada tanggal ini. Hasil kosong bukan bukti rekonsiliasi bersih.</div> : !data.run_completed_at ? <div className="alert warn" role="status">Worker belum menyelesaikan run. Tunggu hasil selesai sebelum menarik kesimpulan.</div> : <p role="status">Run selesai {new Date(data.run_completed_at).toLocaleString('id-ID')}. Temuan pada bisnis ini: {data.tenant_finding_count}.</p>}
        {data.truncated && <Notice>Hasil dibatasi 1.000 temuan. Jangan menganggap tabel ini daftar lengkap.</Notice>}
        {data.run_completed_at && !data.truncated && data.tenant_finding_count === 0 && <div className="alert success" role="status">Tidak ada temuan pada run ini untuk bisnis Anda. Ini bukan pengganti audit keamanan/keuangan.</div>}
        {rows.length > 0 ? <div className="table-scroll"><table><thead><tr><th>Jenis</th><th>Kunci entitas</th><th>Diharapkan</th><th>Aktual</th><th>Aktual − diharapkan</th></tr></thead><tbody>{rows.map((row,index) => <tr key={`${row.kind}-${row.entity_key}-${index}`}><td>{row.kind}</td><td style={{ overflowWrap:'anywhere' }}>{row.entity_key}</td><td>{row.expected_value}</td><td>{row.actual_value}</td><td>{decimalDifference(row.expected_value,row.actual_value)}</td></tr>)}</tbody></table></div> : <p className="muted">Tidak ada baris pada filter yang sedang ditampilkan.</p>}
        <p className="muted">Angka ditampilkan dalam unit internal sumber (sen atau milli-unit), bukan otomatis Rupiah. Halaman ini hanya baca; perbaikan harus lewat dokumen koreksi yang sesuai.</p>
      </>}</ResourceState>
    </section>
  </>;
}
