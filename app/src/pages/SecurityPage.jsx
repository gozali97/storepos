import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useSession } from '../auth/SessionContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { PageHeading, Notice, ResourceState } from '../components/UI.jsx';

export default function SecurityPage() {
  const { user } = useSession();
  return <AccountSessions key={user?.id} />;
}

function AccountSessions() {
  const { client } = useSession();
  const [cursor, setCursor] = useState('');
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');
  const lock = useRef(false);
  const resource = useResource(`/me/sessions${cursor ? `?after=${cursor}` : ''}`);
  useEffect(() => {
    const timer = setInterval(resource.reload, 60000);
    return () => clearInterval(timer);
  }, [resource.reload]);
  const time = value => new Date(value).toLocaleString('id-ID');
  async function revoke() {
    if (lock.current) return;
    lock.current = true; setBusy(true); setMessage(''); setSuccess('');
    try {
      const result = await client.post('/me/sessions/revoke-others', {});
      setSuccess(`${result.revoked} sesi lain dicabut. Sesi yang sedang dipakai tetap aktif.`);
      setConfirm(false); setCursor(''); resource.reload();
    } catch (error) {
      setMessage(!error.status || error.status >= 500 ? 'Status pencabutan belum pasti. Muat ulang daftar; aksi ini dapat diulang karena tidak mencabut sesi saat ini.' : error.message);
      resource.reload();
    } finally { lock.current = false; setBusy(false); }
  }
  return <>
    <PageHeading title="Keamanan akun" description="Daftar sesi akun Anda sendiri, tidak dibatasi pilihan bisnis aktif."><Link to="/pengaturan">Ganti kata sandi</Link></PageHeading>
    <Notice>{message}</Notice><Notice kind="success">{success}</Notice>
    <section className="panel"><div className="table-toolbar"><h2>Sesi aktif</h2><button disabled={busy || resource.loading} onClick={resource.reload}>Muat ulang</button><button disabled={busy} onClick={() => setConfirm(true)}>Cabut seluruh sesi lain</button></div>
      <p className="muted">Alamat IP dan browser membantu mengenali sesi, bukan bukti pasti identitas/perangkat. Hanya sesi aktif yang belum kedaluwarsa ditampilkan. Waktu menggunakan zona perangkat ini.</p>
      {confirm && <form onSubmit={event => { event.preventDefault(); revoke(); }}><fieldset disabled={busy}><p>Semua perangkat lain harus login ulang. Sesi saat ini tidak dicabut.</p><label><input type="checkbox" required /> Saya mengonfirmasi pencabutan seluruh sesi lain</label><div className="row"><button className="primary">Konfirmasi cabut sesi lain</button><button type="button" onClick={() => setConfirm(false)}>Batal</button></div></fieldset></form>}
      <ResourceState resource={resource} empty={!resource.data?.sessions?.length}><div className="table-scroll"><table><thead><tr><th>Sesi</th><th>Browser</th><th>IP</th><th>Login</th><th>Terakhir aktif</th><th>Kedaluwarsa</th></tr></thead><tbody>{resource.data?.sessions?.map(s => <tr key={s.session_id}><td>{s.current ? <strong>Sesi ini</strong> : 'Perangkat lain'}<small className="table-subtext">{s.session_id}</small></td><td style={{ overflowWrap: 'anywhere' }}>{s.user_agent || 'Tidak tersedia'}</td><td>{s.ip || 'Tidak tersedia'}</td><td>{time(s.created_at)}</td><td>{time(s.last_seen_at)}</td><td>{time(s.expires_at)}</td></tr>)}</tbody></table></div></ResourceState>
      <div className="row"><button disabled={busy || !cursor} onClick={() => setCursor('')}>Halaman pertama</button><button disabled={busy || resource.loading || !resource.data?.next_cursor} onClick={() => setCursor(resource.data.next_cursor)}>Berikutnya</button></div>
    </section>
  </>;
}
