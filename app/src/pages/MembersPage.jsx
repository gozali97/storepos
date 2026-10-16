import { useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { PageHeading, Notice, ResourceState, Field } from '../components/UI.jsx';

const roles = { owner: 'Pemilik', manager: 'Manajer', cashier: 'Kasir', warehouse: 'Gudang', kitchen: 'Dapur', pharmacist: 'Apoteker' };

function BranchSelection({ branches, selected = [] }) {
  const missing = selected.filter(id => !branches.some(branch => branch.id === id));
  return <fieldset className="member-branches"><legend>Penugasan cabang</legend>
    <div className="checkbox-row">{branches.map(branch => <label key={branch.id}><input type="checkbox" name="branch_ids" value={branch.id} defaultChecked={selected.includes(branch.id)} />{branch.name}</label>)}</div>
    {missing.map(id => <label key={id}><input type="checkbox" name="branch_ids" value={id} defaultChecked />Cabang {id}</label>)}
    {!branches.length && !missing.length && <p className="muted small">Belum ada cabang yang dapat ditugaskan.</p>}
  </fieldset>;
}

export default function MembersPage() {
  const { client, business, user, muatUlang } = useSession();
  const { branches } = useWorkspace();
  const managing = ['owner', 'manager'].includes(business?.role);
  const roleOptions = Object.entries(roles).filter(([role]) => business?.role === 'owner' || !['owner', 'manager'].includes(role));
  const [cursor, setCursor] = useState('');
  const [editing, setEditing] = useState(null);
  const [invitation, setInvitation] = useState(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');
  const resource = useResource(managing ? `/memberships${cursor ? `?after=${encodeURIComponent(cursor)}` : ''}` : null);
  const members = resource.data?.memberships ?? [];

  async function saveMember(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true); setMessage(''); setSuccess('');
    try {
      await client.put(`/memberships/${editing.membership_id}`, {
        role: form.get('role'), status: form.get('status'), branch_ids: form.getAll('branch_ids'),
      }, { version: editing.version });
      setEditing(null); resource.reload();
      setSuccess('Peran, status, dan cabang anggota disimpan.');
      if (editing.user_id === user?.id) await muatUlang();
    } catch (error) {
      setMessage(error.message);
    } finally { setBusy(false); }
  }

  async function invite(event) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    setBusy(true); setMessage(''); setSuccess('');
    try {
      const result = await client.post('/invitations', { email: String(form.get('email')).trim(), role: form.get('role'), branch_ids: form.getAll('branch_ids') });
      setInvitation(result); formElement.reset();
      setSuccess('Undangan dibuat. Bagikan token kepada pemilik email yang diundang.');
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }

  async function revoke() {
    setBusy(true); setMessage(''); setSuccess('');
    try {
      await client.del(`/invitations/${invitation.invitation_id}`);
      setInvitation(null); setSuccess('Undangan dicabut.');
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }

  if (!managing) return <section className="panel empty-state"><h1>Akses dibatasi</h1><p>Pengelolaan anggota hanya tersedia untuk Pemilik dan Manajer.</p></section>;
  return <>
    <PageHeading title="Anggota" description="Kelola peran, akses cabang, dan status anggota bisnis." />
    <Notice>{message}</Notice><Notice kind="success">{success}</Notice>
    {editing && <section className="panel form-panel"><h2>Ubah akses anggota</h2><p className="muted small">ID akun {editing.user_id}. Perubahan peran atau penonaktifan akan menghapus PIN otorisasi anggota.</p>
      <form onSubmit={saveMember} key={`${editing.membership_id}-${editing.version}`}><fieldset disabled={busy}><div className="form-grid">
        <Field label="Peran" id="member_role"><select id="member_role" name="role" defaultValue={editing.role}>{roleOptions.map(([role, name]) => <option key={role} value={role}>{name}</option>)}</select></Field>
        <Field label="Status" id="member_status"><select id="member_status" name="status" defaultValue={editing.status}><option value="active">Aktif</option><option value="disabled">Nonaktif</option></select></Field>
      </div><BranchSelection branches={branches} selected={editing.branch_ids} /><div className="row"><button className="primary">Simpan akses</button><button type="button" onClick={() => setEditing(null)}>Batal</button></div></fieldset></form>
    </section>}
    <section className="panel form-panel"><div className="table-toolbar"><h2>Daftar anggota</h2><button disabled={busy} onClick={resource.reload}>Muat ulang</button></div>
      <ResourceState resource={resource} empty={!members.length}><div className="table-scroll"><table><thead><tr><th>ID akun</th><th>Peran</th><th>Status</th><th>Cabang</th><th>Aksi</th></tr></thead><tbody>{members.map(member => <tr key={member.membership_id}><td className="member-id">{member.user_id}{member.user_id === user?.id && <small className="table-subtext">Akun Anda</small>}</td><td>{roles[member.role] ?? member.role}</td><td>{member.status === 'active' ? 'Aktif' : 'Nonaktif'}</td><td>{member.role === 'owner' ? 'Seluruh cabang' : member.branch_ids.map(id => branches.find(branch => branch.id === id)?.name ?? id).join(', ') || 'Belum ditugaskan'}</td><td><button disabled={busy} onClick={() => setEditing(member)}>Ubah akses</button></td></tr>)}</tbody></table></div></ResourceState>
      <div className="table-footer"><span>{members.length} anggota</span><div className="row">{cursor && <button onClick={() => setCursor('')}>Halaman pertama</button>}<button disabled={!resource.data?.next_cursor || busy} onClick={() => setCursor(resource.data.next_cursor)}>Berikutnya</button></div></div>
    </section>
    <section className="panel form-panel"><h2>Undang anggota</h2><p className="muted small">Undangan berlaku 7 hari. Pengiriman dilakukan secara manual; penerima harus masuk memakai akun dengan email yang diundang.</p>
      <form onSubmit={invite}><fieldset disabled={busy || !!invitation}><div className="form-grid"><Field label="Email penerima" id="invitation_email"><input id="invitation_email" name="email" type="email" required maxLength={254} /></Field><Field label="Peran" id="invitation_role"><select id="invitation_role" name="role" defaultValue="cashier">{roleOptions.map(([role, name]) => <option key={role} value={role}>{name}</option>)}</select></Field></div><BranchSelection branches={branches} /><button className="primary">Buat undangan</button></fieldset></form>
      {invitation && <div className="invitation-result"><label>Token undangan<input readOnly value={invitation.token} onFocus={event => event.target.select()} /></label><p className="muted small">Berlaku sampai {new Date(invitation.expires_at).toLocaleString('id-ID')}. Token hanya ditampilkan di halaman ini.</p><div className="row"><button className="danger" disabled={busy} onClick={revoke}>Cabut undangan</button><button disabled={busy} onClick={() => setInvitation(null)}>Selesai</button></div></div>}
    </section>
  </>;
}
