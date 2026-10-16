import { useEffect, useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { Field, Notice } from '../components/UI.jsx';

export default function KitchenSettings() {
  const { businessId } = useSession();
  const { branch } = useWorkspace();
  return branch ? <KitchenRouting key={`${businessId}.${branch.id}`} branch={branch} /> : null;
}

function KitchenRouting({ branch }) {
  const { client } = useSession();
  const [data, setData] = useState(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');
  async function load(signal) {
    async function all(path, field) {
      let rows = [], cursor = '';
      do {
        const result = await client.get(path + (cursor ? `&after=${encodeURIComponent(cursor)}` : ''), { signal });
        rows = rows.concat(result[field] ?? []);
        cursor = result.next_cursor || '';
      } while (cursor);
      return rows;
    }
    const [categories, stations, routes] = await Promise.all([
      client.get('/categories', { signal }),
      all(`/restaurant/stations?branch_id=${branch.id}`, 'stations'),
      all(`/restaurant/category-routes?branch_id=${branch.id}`, 'routes'),
    ]);
    setData({ categories: categories.categories ?? [], stations: stations.filter(item => !item.archived), routes });
  }
  useEffect(() => {
    const controller = new AbortController();
    load(controller.signal).catch(error => { if (!controller.signal.aborted) setMessage(error.message); });
    return () => controller.abort();
  }, []);
  async function save(event, category) {
    event.preventDefault();
    const form = event.currentTarget;
    const values = new FormData(form);
    setBusy(true); setMessage(''); setSuccess('');
    try {
      if (category) {
        const route = data.routes.find(item => item.category_id === category.id);
        await client.put(`/restaurant/category-routes/${category.id}`, { branch_id: branch.id, station_id: values.get('station_id') }, { version: route?.version ?? 0 });
      } else {
        await client.post('/restaurant/stations', { branch_id: branch.id, name: values.get('station_name').trim() });
        form.reset();
      }
      await load();
      setSuccess(category ? `Kategori ${category.name} sudah diarahkan ke dapur.` : 'Stasiun dapur dibuat. Pilih stasiun untuk setiap kategori di bawah.');
    } catch (error) { setMessage(error.message); if (error.status === 409) await load().catch(() => {}); }
    finally { setBusy(false); }
  }
  return <section className="panel form-panel" id="pengaturan-dapur">
    <h2>Stasiun & kategori dapur</h2><p className="muted small">Cabang {branch.name}. Setiap kategori menu perlu diarahkan ke stasiun aktif sebelum pesanan dikirim.</p>
    <Notice>{message}</Notice><Notice kind="success">{success}</Notice>
    {!data ? message ? <button onClick={() => { setMessage(''); load().catch(error => setMessage(error.message)); }}>Muat ulang pengaturan dapur</button> : <p role="status">Memuat pengaturan dapur…</p> : <>
      <form onSubmit={event => save(event)}><fieldset disabled={busy}>
        <Field label="Nama stasiun baru" id="station_name" name="station_name" required maxLength={100} placeholder="Contoh: Dapur makanan atau Bar minuman" />
        <button className="primary">Tambah stasiun</button>
      </fieldset></form>
      {!data.stations.length && <p className="alert warn">Belum ada stasiun aktif. Tambahkan stasiun terlebih dahulu.</p>}
      {data.categories.map(category => {
        const route = data.routes.find(item => item.category_id === category.id);
        return <form key={`${category.id}.${route?.version ?? 0}`} onSubmit={event => save(event, category)}><fieldset disabled={busy || !data.stations.length}>
          <Field label={`Stasiun untuk ${category.name}`} id={`route-${category.id}`}><select id={`route-${category.id}`} name="station_id" defaultValue={route?.station_id ?? ''} required><option value="">Pilih stasiun dapur</option>{data.stations.map(station => <option key={station.station_id} value={station.station_id}>{station.name}</option>)}</select></Field>
          <button>Simpan rute {category.name}</button>
        </fieldset></form>;
      })}
    </>}
  </section>;
}
