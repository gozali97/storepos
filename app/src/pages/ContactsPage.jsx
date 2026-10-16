import { useState } from "react";
import { Link } from 'react-router-dom';
import { useSession } from "../auth/SessionContext.jsx";
import { useResource } from "../hooks/useResource.js";
import { PageHeading, ResourceState, Field, Notice } from "../components/UI.jsx";
import { Icon } from "../components/Icon.jsx";
export default function ContactsPage({
  kind
}) {
  const {
    client,
    business
  } = useSession();
  const supplier = kind === "suppliers";
  const title = supplier ? "Supplier" : "Pelanggan";
  const [cursor, setCursor] = useState("");
  const [query, setQuery] = useState('');
  const [editing, setEditing] = useState(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [success, setSuccess] = useState("");
  const params = new URLSearchParams({ ...(cursor ? { after: cursor } : {}), ...(!supplier && query ? { q: query } : {}) });
  const resource = useResource(`/${kind}?${params}`);
  const items = resource.data?.[kind] ?? [];
  async function save(event) {
    event.preventDefault();
    setBusy(true);
    setMessage("");
    setSuccess("");
    const values = Object.fromEntries(new FormData(event.currentTarget));
    values.name = values.name.trim();
    if (supplier) values.archived = editing?.archived ?? false;
    try {
      if (editing?.id) await client.put(`/${kind}/${editing.id}`, values, {
        version: editing.version
      });else await client.post(`/${kind}`, values);
      setEditing(null);
      resource.reload();
      setSuccess(`${title} berhasil disimpan.`);
    } catch (error) {
      setMessage(error.status === 409 ? "Data berubah atau sudah dipakai. Muat ulang daftar sebelum mencoba lagi." : error.message);
    } finally {
      setBusy(false);
    }
  }
  async function remove(item) {
    if (!window.confirm(supplier ? `Arsipkan supplier ${item.name}? Histori penerimaan tetap tersimpan.` : `Anonimkan ${item.name}? Nama dan nomor telepon dihapus. Tindakan ini tidak dapat dibatalkan.`)) return;
    setBusy(true);
    setMessage("");
    setSuccess("");
    try {
      if (supplier) await client.put(`/suppliers/${item.id}`, {
        name: item.name,
        contact: item.contact,
        address: item.address,
        note: item.note,
        archived: true
      }, {
        version: item.version
      });else await client.del(`/customers/${item.id}`, {
        version: item.version
      });
      resource.reload();
      setSuccess(supplier ? "Supplier diarsipkan." : "Data pelanggan dianonimkan.");
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }
  return <>
  <PageHeading title={title} description={supplier ? "Catat mitra pemasok dan kontak untuk penerimaan barang." : "Simpan kontak pelanggan seperlunya untuk transaksi."}><button className="primary" onClick={() => setEditing(editing ? null : {})} disabled={busy}>
      <Icon name={editing ? "close" : "plus"} size={17} />
      {editing ? "Tutup formulir" : `Tambah ${title.toLowerCase()}`}
    </button></PageHeading>
  <Notice>{message}</Notice>
  <Notice kind="success">{success}</Notice>
  {editing && <section className="panel form-panel">
    <h2>{editing.id ? `Edit ${title.toLowerCase()}` : `Tambah ${title.toLowerCase()}`}</h2>
    <form onSubmit={save} key={editing.id ?? "new"}>
      <div className="form-grid">
        <Field label="Nama" id="name" required maxLength={200} defaultValue={editing.name ?? ""} />
        <Field label={supplier ? "Kontak" : "Nomor telepon"} id={supplier ? "contact" : "phone"} maxLength={supplier ? 200 : 32} defaultValue={supplier ? editing.contact ?? "" : editing.phone ?? ""} />
        {supplier && <Field label="Alamat" id="address" maxLength={1e3} defaultValue={editing.address ?? ""} />}
        <Field label="Catatan" id="note" maxLength={supplier ? 1e3 : 200} defaultValue={editing.note ?? ""} />
      </div>
      <button className="primary" disabled={busy}>{busy ? "Menyimpan…" : "Simpan"}</button>
    </form>
  </section>}
  <section className="panel">
    <div className="panel-heading">
      <h2>Daftar {title.toLowerCase()}</h2>
      <button className="icon-button" aria-label={`Muat ulang ${title}`} onClick={resource.reload}><Icon name="refresh" /></button>
    </div>
    {!supplier && <Field label="Cari nama atau nomor telepon" id="customer_search" value={query} onChange={event => { setQuery(event.target.value); setCursor(''); }} maxLength={100} />}
    <ResourceState resource={resource} empty={!items.length} message={`Tambahkan ${title.toLowerCase()} pertama melalui tombol di atas.`}><div className="table-scroll"><table>
          <thead><tr>
              <th>Nama</th>
              <th>Kontak</th>
              {supplier && <th>Alamat</th>}
              <th>Catatan</th>
              <th>Aksi</th>
            </tr></thead>
          <tbody>{items.map(item => <tr key={item.id}>
              <td>
                <span className="product-name">
                  <span className="product-avatar"><Icon name={supplier ? "truck" : "users"} /></span>
                  <strong>{item.name}</strong>
                </span>
                {item.archived && <span className="status-badge">Diarsipkan</span>}
              </td>
              <td>{item.contact || item.phone || "—"}</td>
              {supplier && <td>{item.address || "—"}</td>}
              <td>{item.note || "—"}</td>
              <td><div className="row">
                  {!supplier && !item.anonymized && <Link to={`/transaksi?customer_id=${encodeURIComponent(item.id)}`}>Riwayat transaksi</Link>}
                  <button disabled={busy} onClick={() => {
                      setEditing(item);
                      setMessage("");
                    }}>Edit</button>
                  {(supplier || business?.role === "owner") && !item.archived && <button className="danger" disabled={busy} onClick={() => remove(item)}>{supplier ? "Arsipkan" : "Anonimkan"}</button>}
                </div></td>
            </tr>)}</tbody>
        </table></div></ResourceState>
    <div className="table-footer">
      <span>{items.length} kontak pada halaman ini</span>
      <div className="row">
        {cursor && <button onClick={() => setCursor("")}>Halaman pertama</button>}
        <button disabled={!resource.data?.next_cursor || resource.loading} onClick={() => setCursor(resource.data.next_cursor)}>Berikutnya</button>
      </div>
    </div>
  </section>
</>;
}
