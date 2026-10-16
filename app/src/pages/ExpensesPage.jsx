import { useEffect, useRef, useState } from "react";
import { useSession } from "../auth/SessionContext.jsx";
import { useWorkspace } from "../workspace/WorkspaceContext.jsx";
import { useResource } from "../hooks/useResource.js";
import { newIdempotencyKey } from "../api/client.js";
import { rupiahToCents } from "../forms.js";
import { optionalApproval } from "../cashier.js";
import { formatIDR } from "../money.js";
import { localDate } from "../reporting.js";
import { PageHeading, Field, Notice, ResourceState } from "../components/UI.jsx";
import { Icon } from "../components/Icon.jsx";
export default function ExpensesPage() {
  const {
    client,
    businessId,
    user,
    business
  } = useSession();
  const {
    branch,
    register
  } = useWorkspace();
  const [from, setFrom] = useState(localDate(branch?.timezone));
  const [to, setTo] = useState(localDate(branch?.timezone));
  const [cursor, setCursor] = useState("");
  useEffect(() => setCursor(''), [branch?.id]);
  const [show, setShow] = useState(false);
  const [method, setMethod] = useState(business?.role === 'cashier' ? 'shift_cash' : 'outside_cash');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [success, setSuccess] = useState("");
  const storageKey = `storepos.pendingExpense.${businessId}.${user?.id}`;
  const [pending, setPending] = useState(() => {
    try {
      return JSON.parse(sessionStorage.getItem(storageKey)) || null;
    } catch {
      return null;
    }
  });
  const form = useRef(null);
  const params = new URLSearchParams({
    branch_id: branch?.id ?? "",
    from,
    to,
    ...(cursor ? {
      after: cursor
    } : {})
  });
  const resource = useResource(branch && from && to && from <= to ? `/expenses?${params}` : null);
  const items = resource.data?.expenses ?? [];
  async function send(operation) {
    if (busy) return;
    setBusy(true);
    setMessage("");
    setSuccess("");
    try {
      await client.post("/expenses", operation.body, {
        idempotencyKey: operation.key
      });
      sessionStorage.removeItem(storageKey);
      setPending(null);
      setShow(false);
      form.current?.reset();
      resource.reload();
      setSuccess("Pengeluaran berhasil dicatat.");
    } catch (error) {
      setMessage(error.message);
      if (error.status && error.status < 500) {
        sessionStorage.removeItem(storageKey);
        setPending(null);
      }
    } finally {
      setBusy(false);
    }
  }
  function create(event) {
    event.preventDefault();
    const values = Object.fromEntries(new FormData(event.currentTarget));
    try {
      const body = {
        branch_id: branch.id,
        expense_date: values.expense_date,
        category: values.category.trim(),
        amount_cents: rupiahToCents(values.amount),
        method,
        note: values.note.trim(),
        ...(method === 'shift_cash' ? { register_id: register?.id } : {}),
        approval: optionalApproval(values.approval_email, values.approval_pin)
      };
      if (method === 'shift_cash' && !register) throw new Error('Pilih terminal untuk mencatat pengeluaran kas shift.');
      const operation = {
        body,
        key: newIdempotencyKey()
      };
      sessionStorage.setItem(storageKey, JSON.stringify(operation));
      setPending(operation);
      send(operation);
    } catch (error) {
      setMessage(error.message);
    }
  }
  return <>
  <PageHeading title="Pengeluaran" description="Catat biaya operasional dari kas shift atau dana di luar kas."><button className="primary" disabled={!branch || !!pending || busy} onClick={() => setShow(v => !v)}>
      <Icon name={show ? "close" : "plus"} size={17} />
      {show ? "Tutup formulir" : "Catat pengeluaran"}
    </button></PageHeading>
  <Notice>{message}</Notice>
  <Notice kind="success">{success}</Notice>
  {pending && <div className="alert warn" role="status">
    <p>{busy ? "Mencatat pengeluaran…" : "Status pengeluaran belum pasti. Periksa kembali dengan kunci yang sama agar tidak tercatat dua kali."}</p>
    <button disabled={busy} onClick={() => send(pending)}>Periksa & coba lagi</button>
  </div>}
  {show && <section className="panel form-panel">
    <h2>Pengeluaran baru</h2>
    <form ref={form} onSubmit={create}><fieldset disabled={busy || !!pending}>
        <div className="form-grid">
          <Field label="Tanggal" id="expense_date" type="date" required defaultValue={localDate(branch?.timezone)} />
          <Field label="Kategori" id="category" required maxLength={100} placeholder="Listrik, sewa, transportasi…" />
          <Field label="Nominal (Rp)" id="amount" type="number" step="0.01" min="0.01" required />
          <Field label="Catatan" id="note" maxLength={200} />
          <Field label="Sumber dana" id="expense_method"><select id="expense_method" value={method} onChange={event => setMethod(event.target.value)}>
            {business?.role !== 'cashier' && <option value="outside_cash">Di luar kas shift</option>}
            <option value="shift_cash">Kas shift terminal aktif</option>
          </select></Field>
          {method === 'shift_cash' && <p className="muted small">Terminal: {register?.name ?? 'Belum dipilih'}. Pengeluaran akan mengurangi kas shift yang terbuka.</p>}
          <Field label="Email atasan (bila diminta)" id="approval_email" type="email" required={business?.role === 'cashier'} autoComplete="username" />
          <Field label="PIN atasan (6 digit)" id="approval_pin" type="password" inputMode="numeric" pattern="[0-9]{6}" maxLength={6} required={business?.role === 'cashier'} autoComplete="off" />
        </div>
        <button className="primary">Simpan pengeluaran</button>
      </fieldset></form>
  </section>}
  <section className="panel">
    <div className="table-toolbar">
      <h2>Catatan pengeluaran</h2>
      <div className="filter-fields">
        <label>Dari<input type="date" value={from} max={to} onChange={e => {
              setFrom(e.target.value);
              setCursor("");
            }} /></label>
        <label>Sampai<input type="date" value={to} min={from} onChange={e => {
              setTo(e.target.value);
              setCursor("");
            }} /></label>
      </div>
    </div>
    {!branch ? <div className="empty-state">Pilih cabang untuk melihat pengeluaran.</div> : from > to ? <Notice>Tanggal awal harus sebelum tanggal akhir.</Notice> : <ResourceState resource={resource} empty={!items.length} message="Catat biaya operasional pertama atau pilih periode lain."><div className="table-scroll"><table>
          <thead><tr>
              <th>Tanggal</th>
              <th>Kategori</th>
              <th>Metode</th>
              <th>Catatan</th>
              <th className="num">Nominal</th>
            </tr></thead>
          <tbody>{items.map(item => <tr key={item.expense_id}>
              <td>{item.expense_date}</td>
              <td><strong>{item.category}</strong></td>
              <td>{item.method === "shift_cash" ? "Kas shift" : "Di luar kas"}</td>
              <td>{item.note || "—"}</td>
              <td className="num">{formatIDR(BigInt(item.amount_cents))}</td>
            </tr>)}</tbody>
        </table></div></ResourceState>}
    <div className="table-footer">
      <span>{items.length} pengeluaran</span>
      <div className="row">
        {cursor && <button onClick={() => setCursor("")}>Halaman pertama</button>}
        <button disabled={!resource.data?.next_cursor} onClick={() => setCursor(resource.data.next_cursor)}>Berikutnya</button>
      </div>
    </div>
  </section>
</>;
}
