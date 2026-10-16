import { useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { Field, Notice, ResourceState } from '../components/UI.jsx';
import { rupiahToCents } from '../forms.js';
import { discountToInput } from '../cashier.js';

function BranchFields({ value = {}, editing = false }) {
  return <div className="form-grid">
    <Field label="Kode cabang" id={editing ? 'edit_code' : 'new_code'}><input id={editing ? 'edit_code' : 'new_code'} name="code" defaultValue={value.code ?? ''} required pattern="[A-Z0-9-]+" maxLength={20} readOnly={editing} /></Field>
    <Field label="Nama cabang" id={editing ? 'edit_name' : 'new_name'}><input id={editing ? 'edit_name' : 'new_name'} name="name" defaultValue={value.name ?? ''} required maxLength={200} /></Field>
    <Field label="Zona waktu IANA" id={editing ? 'edit_timezone' : 'new_timezone'}><input id={editing ? 'edit_timezone' : 'new_timezone'} name="timezone" defaultValue={value.timezone ?? 'Asia/Jakarta'} required placeholder="Asia/Jakarta" /></Field>
    <Field label="Alamat" id={editing ? 'edit_address' : 'new_address'}><textarea id={editing ? 'edit_address' : 'new_address'} name="address" defaultValue={value.address ?? ''} maxLength={1000} rows={2} /></Field>
    <Field label="Header struk" id={editing ? 'edit_header' : 'new_header'}><textarea id={editing ? 'edit_header' : 'new_header'} name="receipt_header" defaultValue={value.receipt_header ?? ''} maxLength={1000} rows={2} /></Field>
    <Field label="Footer struk" id={editing ? 'edit_footer' : 'new_footer'}><textarea id={editing ? 'edit_footer' : 'new_footer'} name="receipt_footer" defaultValue={value.receipt_footer ?? ''} maxLength={1000} rows={2} /></Field>
  </div>;
}

export default function OrganizationSettings() {
  const { businessId, user } = useSession();
  const { branch } = useWorkspace();
  return <OrganizationWorkspace key={`${businessId}.${user?.id}.${branch?.id}`} />;
}

function OrganizationWorkspace() {
  const { client } = useSession();
  const workspace = useWorkspace();
  const branch = useResource(workspace.branch ? `/branches/${workspace.branch.id}` : null);
  const register = useResource(workspace.register ? `/registers/${workspace.register.id}` : null);
  const policy = useResource('/settings/operational-policy');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');
  const [inactiveRegister, setInactiveRegister] = useState(null);
  const [registerCursor, setRegisterCursor] = useState('');
  const allRegisters = useResource(workspace.branch ? `/branches/${workspace.branch.id}/registers?status=all${registerCursor ? `&after=${registerCursor}` : ''}` : null);

  async function submit(event, action, reload, successMessage) {
    event.preventDefault();
    const form = event.currentTarget;
    const values = Object.fromEntries(new FormData(form));
    setBusy(true);
    setMessage('');
    setSuccess('');
    try {
      await action(values);
      reload?.();
      allRegisters.reload();
      workspace.reload();
      setSuccess(successMessage);
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }

  async function changeRegisterStatus(item) {
    if (busy) return;
    setBusy(true);
    setMessage('');
    setSuccess('');
    const status = item.status === 'active' ? 'inactive' : 'active';
    try {
      const result = await client.patch(`/registers/${item.register_id}/status`, { status }, { version: item.version });
      setInactiveRegister(status === 'inactive' ? { ...item, ...result } : null);
      register.reload();
      allRegisters.reload();
      workspace.reload();
      setSuccess(`Terminal ${item.name} ${status === 'active' ? 'diaktifkan' : 'dinonaktifkan'}.`);
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }

  return <>
    <Notice>{message}</Notice><Notice kind="success">{success}</Notice>
    <div className="settings-grid">
      <section className="panel form-panel"><h2>Kebijakan operasional</h2><p className="muted small">Batas persetujuan untuk diskon kasir, kas keluar, dan selisih tutup shift.</p>
        <ResourceState resource={policy}>{policy.data && <form key={policy.data.version} onSubmit={event => submit(event, values => client.put('/settings/operational-policy', {
          ...policy.data.policy,
          return_window_days: Number(values.return_window_days),
          cash_rounding_enabled: values.rounding_enabled === 'on',
          cash_rounding_multiple_cents: rupiahToCents(values.rounding_multiple),
          cashier_discount_bps: Number(rupiahToCents(values.discount)),
          cash_out_approval_cents: rupiahToCents(values.cash_out),
          variance_approval_cents: rupiahToCents(values.variance),
        }, { version: policy.data.version }), policy.reload, 'Kebijakan operasional disimpan.')}>
          <fieldset disabled={busy}><div className="checkbox-row"><label><input name="rounding_enabled" type="checkbox" defaultChecked={policy.data.policy.cash_rounding_enabled} />Aktifkan pembulatan pembayaran tunai</label></div>
            <Field label="Kelipatan pembulatan tunai (Rp)" id="rounding_multiple" type="number" min="1" max="1000" step="1" required defaultValue={discountToInput({ kind: 'amount', value: policy.data.policy.cash_rounding_multiple_cents })} />
            <Field label="Batas diskon kasir (%)" id="discount" type="number" min="0" max="100" step="0.01" required defaultValue={discountToInput({ kind: 'percent', value: String(policy.data.policy.cashier_discount_bps) })} />
            <Field label="Batas hari retur (0 = tanpa batas)" id="return_window_days" type="number" min="0" max="3650" step="1" required defaultValue={policy.data.policy.return_window_days ?? 0} />
            <Field label="Ambang persetujuan kas keluar (Rp)" id="cash_out" type="number" min="0" step="0.01" required defaultValue={discountToInput({ kind: 'amount', value: policy.data.policy.cash_out_approval_cents })} />
            <Field label="Ambang persetujuan selisih shift (Rp)" id="variance" type="number" min="0" step="0.01" required defaultValue={discountToInput({ kind: 'amount', value: policy.data.policy.variance_approval_cents })} />
            <button className="primary">Simpan kebijakan</button></fieldset>
        </form>}</ResourceState>
      </section>
      <section className="panel form-panel"><h2>Tambah cabang</h2><form onSubmit={event => submit(event, values => client.post('/branches', values), null, 'Cabang baru ditambahkan.')}><fieldset disabled={busy}><BranchFields /><button className="primary">Tambah cabang</button></fieldset></form></section>
      <section className="panel form-panel"><h2>Cabang aktif & struk</h2>{!workspace.branch ? <p>Pilih cabang terlebih dahulu.</p> : <ResourceState resource={branch}>{branch.data && <form key={`${branch.data.branch_id}-${branch.data.version}`} onSubmit={event => submit(event, values => client.put(`/branches/${branch.data.branch_id}`, values, { version: branch.data.version }), branch.reload, 'Cabang dan teks struk disimpan.')}><fieldset disabled={busy}><BranchFields value={branch.data} editing /><p className="muted small">Zona waktu terkunci setelah penjualan pertama.</p><button className="primary">Simpan cabang</button></fieldset></form>}</ResourceState>}</section>
      <section className="panel form-panel"><h2>Terminal cabang aktif</h2>{workspace.branch && <form onSubmit={event => submit(event, values => client.post(`/branches/${workspace.branch.id}/registers`, values), register.reload, 'Terminal baru ditambahkan.')}><fieldset disabled={busy}><Field label="Nama terminal baru" id="register_name"><input id="register_name" name="name" required maxLength={200} /></Field><button className="primary">Tambah terminal</button></fieldset></form>}
        {workspace.register && <ResourceState resource={register}>{register.data && <div className="table-footer"><span>{register.data.name} · {register.data.status === 'active' ? 'Aktif' : 'Nonaktif'}</span><button className="danger" disabled={busy} onClick={() => changeRegisterStatus(register.data)}>Nonaktifkan terminal</button></div>}</ResourceState>}
        <ResourceState resource={allRegisters}><div className="table-scroll"><table><thead><tr><th>Terminal tersimpan</th><th>Status</th><th>Aksi</th></tr></thead><tbody>{allRegisters.data?.registers?.map(item => <tr key={item.register_id}><td>{item.name}</td><td>{item.status}</td><td><button disabled={busy || item.status === 'archived'} onClick={() => changeRegisterStatus(item)}>{item.status === 'active' ? 'Nonaktifkan' : 'Aktifkan'}</button></td></tr>)}</tbody></table></div></ResourceState>
        <div className="row"><button disabled={!registerCursor || busy} onClick={() => setRegisterCursor('')}>Terminal halaman pertama</button><button disabled={busy || allRegisters.loading || !allRegisters.data?.next_cursor} onClick={() => setRegisterCursor(allRegisters.data.next_cursor)}>Terminal berikutnya</button><button disabled={busy} onClick={allRegisters.reload}>Muat ulang terminal</button></div>
        {inactiveRegister && <div className="table-footer"><span>{inactiveRegister.name} · Nonaktif</span><button disabled={busy} onClick={() => changeRegisterStatus(inactiveRegister)}>Aktifkan kembali</button></div>}
      </section>
    </div>
  </>;
}
