import { useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { PageHeading, Field, Notice, ResourceState } from '../components/UI.jsx';

export default function TableManagementPage() {
  const { businessId, user, business } = useSession();
  const { branch } = useWorkspace();
  if (!['owner', 'manager'].includes(business?.role)) return <Notice>Pengelolaan meja hanya untuk Pemilik/Manajer.</Notice>;
  return <TableManagementWorkspace key={`${businessId}.${user?.id}.${branch?.id}`} />;
}

function TableManagementWorkspace() {
  const { client } = useSession();
  const { branch } = useWorkspace();
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');
  const [editingArea, setEditingArea] = useState(null);
  const [editingTable, setEditingTable] = useState(null);
  const [areaCursor, setAreaCursor] = useState('');
  const [tableCursor, setTableCursor] = useState('');
  const areaPath = branch ? `/restaurant/areas?branch_id=${branch.id}&include_archived=true${areaCursor ? `&after=${areaCursor}` : ''}` : null;
  const tablePath = branch ? `/restaurant/tables?branch_id=${branch.id}&include_archived=true${tableCursor ? `&after=${tableCursor}` : ''}` : null;
  const areas = useResource(areaPath);
  const tables = useResource(tablePath);
  const areaItems = areas.data?.areas ?? [];
  const tableItems = tables.data?.tables ?? [];

  async function run(fn, ok) {
    if (busy) return;
    setBusy(true); setMessage(''); setSuccess('');
    try { await fn(); setSuccess(ok); setEditingArea(null); setEditingTable(null); }
    catch (error) { setMessage(error.status === 409 ? 'Data berubah atau masih dipakai. Tampilan dimuat ulang; periksa kembali.' : error.message); }
    finally { setBusy(false); areas.reload(); tables.reload(); }
  }

  function saveArea(event) {
    event.preventDefault();
    const name = new FormData(event.currentTarget).get('name').trim();
    run(() => editingArea
      ? client.put(`/restaurant/areas/${editingArea.area_id}`, { name }, { version: editingArea.version })
      : client.post('/restaurant/areas', { branch_id: branch.id, name }),
    editingArea ? 'Area diperbarui.' : 'Area dibuat.');
  }

  function saveTable(event) {
    event.preventDefault();
    const v = Object.fromEntries(new FormData(event.currentTarget));
    run(() => editingTable
      ? client.put(`/restaurant/tables/${editingTable.table_id}`, { name: v.name.trim(), capacity: Number(v.capacity) }, { version: editingTable.version })
      : client.post('/restaurant/tables', { branch_id: branch.id, area_id: v.area_id, name: v.name.trim(), capacity: Number(v.capacity) }),
    editingTable ? 'Meja diperbarui.' : 'Meja dibuat.');
  }

  function changeArchive(type, item, restore) {
    const plural = type === 'area' ? 'areas' : 'tables';
    run(() => restore
      ? client.post(`/restaurant/${plural}/${item[`${type}_id`]}/restore`, {}, { version: item.version })
      : client.del(`/restaurant/${plural}/${item[`${type}_id`]}`, { version: item.version }),
    `${type === 'area' ? 'Area' : 'Meja'} ${restore ? 'dipulihkan' : 'diarsipkan'}.`);
  }

  return <>
    <PageHeading title="Kelola area & meja" description="Arsipkan master yang tidak dipakai; riwayat pesanan tidak dihapus." />
    <Notice>{message}</Notice><Notice kind="success">{success}</Notice>
    {!branch ? <div className="empty-state">Pilih cabang.</div> : <div className="settings-grid">
      <section className="panel form-panel">
        <h2>{editingArea ? 'Edit area' : 'Area baru'}</h2>
        <form key={editingArea?.area_id ?? 'new-area'} onSubmit={saveArea}><fieldset disabled={busy}>
          <Field label="Nama area" id="area_name"><input id="area_name" name="name" required maxLength={100} defaultValue={editingArea?.name ?? ''} /></Field>
          <div className="row"><button className="primary">{editingArea ? 'Simpan' : 'Buat area'}</button>{editingArea && <button type="button" onClick={() => setEditingArea(null)}>Batal</button>}</div>
        </fieldset></form>
        <ResourceState resource={areas} empty={!areaItems.length}><div className="table-scroll"><table><thead><tr><th>Area</th><th>Status</th><th>Aksi</th></tr></thead><tbody>
          {areaItems.map(a => <tr key={a.area_id}><td>{a.name}</td><td>{a.archived ? 'Arsip' : 'Aktif'}</td><td className="row">{a.archived ? <button disabled={busy} onClick={() => changeArchive('area', a, true)}>Pulihkan</button> : <><button disabled={busy} onClick={() => setEditingArea(a)}>Edit</button><button disabled={busy} onClick={() => changeArchive('area', a, false)}>Arsipkan</button></>}</td></tr>)}
        </tbody></table></div></ResourceState>
        <div className="row"><button disabled={busy || !areaCursor} onClick={() => setAreaCursor('')}>Area halaman pertama</button><button disabled={busy || !areas.data?.next_cursor} onClick={() => setAreaCursor(areas.data.next_cursor)}>Area berikutnya</button></div>
      </section>
      <section className="panel form-panel">
        <h2>{editingTable ? 'Edit meja' : 'Meja baru'}</h2>
        <form key={editingTable?.table_id ?? 'new-table'} onSubmit={saveTable}><fieldset disabled={busy}>
          {!editingTable && <Field label="Area" id="area_id"><select id="area_id" name="area_id" required><option value="">Pilih area</option>{areaItems.filter(a => !a.archived).map(a => <option key={a.area_id} value={a.area_id}>{a.name}</option>)}</select></Field>}
          <Field label="Nama meja" id="table_name"><input id="table_name" name="name" required maxLength={100} defaultValue={editingTable?.name ?? ''} /></Field>
          <Field label="Kapasitas" id="capacity"><input id="capacity" name="capacity" type="number" min="1" max="1000" required defaultValue={editingTable?.capacity ?? 4} /></Field>
          <div className="row"><button className="primary">{editingTable ? 'Simpan' : 'Buat meja'}</button>{editingTable && <button type="button" onClick={() => setEditingTable(null)}>Batal</button>}</div>
        </fieldset></form>
        <ResourceState resource={tables} empty={!tableItems.length}><div className="table-scroll"><table><thead><tr><th>Meja</th><th>Area</th><th>Kapasitas</th><th>Status</th><th>Aksi</th></tr></thead><tbody>
          {tableItems.map(t => <tr key={t.table_id}><td>{t.name}</td><td>{t.area_name}</td><td>{t.capacity}</td><td>{t.archived ? 'Arsip' : t.status}</td><td className="row">{t.archived ? <button disabled={busy} onClick={() => changeArchive('table', t, true)}>Pulihkan</button> : <><button disabled={busy} onClick={() => setEditingTable(t)}>Edit</button><button disabled={busy} onClick={() => changeArchive('table', t, false)}>Arsipkan</button></>}</td></tr>)}
        </tbody></table></div></ResourceState>
        <div className="row"><button disabled={busy || !tableCursor} onClick={() => setTableCursor('')}>Meja halaman pertama</button><button disabled={busy || !tables.data?.next_cursor} onClick={() => setTableCursor(tables.data.next_cursor)}>Meja berikutnya</button></div>
      </section>
    </div>}
  </>;
}
