import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useSession } from '../auth/SessionContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { m2Label, nextStep, progressPercent, shouldShowChecklist, stepInfo } from '../onboarding.js';
import { Icon } from './Icon.jsx';
import { Notice } from './UI.jsx';

// Hanya pemilik yang berhak (business.manage); peran lain menerima 403 dan panel tidak dirender.
export function OnboardingChecklist() {
  const { client } = useSession();
  const resource = useResource('/onboarding');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const data = resource.data;
  if (!shouldShowChecklist(data)) return null;

  async function run(action, done) {
    if (busy) return;
    setBusy(true); setMessage('');
    try { await action(); setMessage(done); } catch (error) { setMessage(error.message); } finally { setBusy(false); resource.reload(); }
  }
  const hide = () => run(() => client.put('/onboarding/dismiss', { dismissed: true }), '');
  const addDemo = () => run(() => client.post('/onboarding/demo', {}), 'Produk contoh ditambahkan.');
  const removeDemo = () => run(() => client.del('/onboarding/demo'), 'Produk contoh dihapus (diarsipkan).');
  const next = nextStep(data.items);
  const percent = progressPercent(data.done, data.total);

  return <section className="panel onboarding" aria-labelledby="onboarding-title">
    <div className="table-toolbar">
      <h2 id="onboarding-title">Mulai berjualan</h2>
      <Link to="/mulai">Buka panduan langkah demi langkah</Link>
      <button onClick={hide} disabled={busy}>Sembunyikan</button>
    </div>
    <div role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent} aria-label="Progres onboarding" className="progress"><span style={{ width: `${percent}%` }} /></div>
    <p className="muted small">{data.done} dari {data.total} langkah selesai.{next ? ` Berikutnya: ${stepInfo(next.key).label}.` : ''}</p>
    <ul className="checklist">{data.items.map(item => {
      const info = stepInfo(item.key);
      return <li key={item.key}>
        <Icon name={item.done ? 'check' : 'clock'} size={17} />
        {item.done ? <span><span className="sr-only">Selesai: </span>{info.label}</span> : <Link to={info.to}>{info.label}</Link>}
      </li>;
    })}</ul>
    <div className="row">
      {data.demo_present
        ? <button onClick={removeDemo} disabled={busy}>Hapus produk contoh</button>
        : <button onClick={addDemo} disabled={busy}>Tambah produk contoh</button>}
    </div>
    {m2Label(data.m2_minutes) && <p className="muted small">{m2Label(data.m2_minutes)}</p>}
    <Notice kind="success">{message}</Notice>
  </section>;
}
