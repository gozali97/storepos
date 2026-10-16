import { useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { Notice, ResourceState } from '../components/UI.jsx';

export default function PaymentMethodSettings() {
  const { client } = useSession();
  const resource = useResource('/settings/payment-methods');
  const [busy, setBusy] = useState(false); const [message, setMessage] = useState(''); const [success, setSuccess] = useState('');
  async function save(event) {
    event.preventDefault(); if (busy) return;
    const v = new FormData(event.currentTarget);
    const methods = resource.data.methods.map(m => ({ code: m.code, name: String(v.get(`name_${m.code}`) ?? '').trim(), enabled: v.has(`enabled_${m.code}`) }));
    if (!methods.some(m => m.enabled)) { setMessage('Minimal satu metode harus aktif.'); return; }
    setBusy(true); setMessage(''); setSuccess('');
    try { await client.put('/settings/payment-methods', { methods }, { version: resource.data.version }); resource.reload(); setSuccess('Metode pembayaran disimpan. Transaksi lama tidak diubah.'); }
    catch (error) { setMessage(error.message); if (error.status === 409) resource.reload(); }
    finally { setBusy(false); }
  }
  return <section className="panel form-panel"><h2>Metode pembayaran manual</h2><p>Ubah nama tampilan atau nonaktifkan metode untuk penjualan baru. Kode historis tetap utuh; refund memakai metode asal. Bukan integrasi pembayaran otomatis.</p><Notice>{message}</Notice><Notice kind="success">{success}</Notice><ResourceState resource={resource}>{resource.data && <form key={resource.data.version} onSubmit={save}><fieldset disabled={busy}>{resource.data.methods.map(m => <div className="form-grid" key={m.code}><label>Nama metode ({m.code})<input name={`name_${m.code}`} required maxLength={100} defaultValue={m.name} /></label><label><input name={`enabled_${m.code}`} type="checkbox" defaultChecked={m.enabled} /> Aktif untuk penjualan baru</label></div>)}<button className="primary">Simpan metode pembayaran</button></fieldset></form>}</ResourceState></section>;
}
