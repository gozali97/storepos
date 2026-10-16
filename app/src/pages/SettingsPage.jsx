import { useState } from "react";
import { Link } from 'react-router-dom';
import { useSession } from "../auth/SessionContext.jsx";
import { useResource } from "../hooks/useResource.js";
import { useWorkspace } from "../workspace/WorkspaceContext.jsx";
import { BUSINESS_TYPE_LABELS } from "../terms.js";
import { PageHeading, Field, Notice, ResourceState } from "../components/UI.jsx";
import { Icon } from "../components/Icon.jsx";
import { rupiahToCents } from "../forms.js";
import { passwordChangeBody } from "../password.js";
import OrganizationSettings from './OrganizationSettings.jsx';
import PaymentMethodSettings from './PaymentMethodSettings.jsx';
import KitchenSettings from './KitchenSettings.jsx';
export default function SettingsPage() {
  const {
    client,
    business,
    user,
    muatUlang,
    updateDisplayName
  } = useSession();
  const {
    branch,
    register,
    branches
  } = useWorkspace();
  const [message, setMessage] = useState("");
  const [success, setSuccess] = useState("");
  const [busy, setBusy] = useState(false);
  const profile = useResource('/me/profile');
  async function acceptInvitation(event) {
    event.preventDefault();
    const token = new FormData(event.currentTarget).get('invitation_token').trim();
    setBusy(true); setMessage(''); setSuccess('');
    try {
      await client.post('/invitations/accept', { token });
      setSuccess('Undangan diterima. Daftar bisnis diperbarui.');
      await muatUlang();
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }
  async function save(event, endpoint, convert) {
    event.preventDefault();
    const form = event.currentTarget;
    const values = Object.fromEntries(new FormData(form));
    if (endpoint.includes("/tax") && !window.confirm("Ganti aturan pajak aktif bisnis? Tarif baru berlaku pada transaksi berikutnya.")) return;
    setBusy(true);
    setMessage("");
    setSuccess("");
    try {
      await client.put(endpoint, convert(values));
      setSuccess("Pengaturan berhasil disimpan.");
      form.reset();
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }
  async function changePassword(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const values = Object.fromEntries(new FormData(form));
    setBusy(true);
    setMessage('');
    setSuccess('');
    try {
      await client.put('/auth/password', passwordChangeBody(values.current_password, values.new_password, values.confirm_password));
      form.reset();
      setSuccess('Kata sandi diperbarui. Sesi di perangkat lain sudah dicabut.');
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }
  async function saveDisplayName(event) {
    event.preventDefault();
    const name = new FormData(event.currentTarget).get('display_name').trim();
    setBusy(true);
    setMessage('');
    setSuccess('');
    try {
      const updated = await client.put('/me/profile', { display_name: name }, { version: profile.data.version });
      updateDisplayName(updated.display_name);
      profile.reload();
      setSuccess('Nama tampilan berhasil diperbarui.');
    } catch (error) {
      if (error.status === 409) profile.reload();
      setMessage(error.status === 409 ? 'Profil berubah di sesi lain. Data terbaru dimuat; periksa kembali sebelum menyimpan.' : error.message);
    } finally {
      setBusy(false);
    }
  }
  return <>
  <PageHeading title="Pengaturan" description="Konteks bisnis, akun, dan pengaturan operasional Anda." />
  <Notice>{message}</Notice>
  <Notice kind="success">{success}</Notice>
  <div className="settings-grid">
    <section className="panel">
      <div className="panel-heading">
        <h2>Profil bisnis</h2>
        <span className="soft-icon"><Icon name="store" /></span>
      </div>
      <dl className="detail-list">
        <div>
          <dt>Nama usaha</dt>
          <dd>{business?.business_name ?? "Belum ada bisnis"}</dd>
        </div>
        <div>
          <dt>Jenis usaha</dt>
          <dd>{BUSINESS_TYPE_LABELS[business?.business_type] ?? "—"}</dd>
        </div>
        <div>
          <dt>Cabang aktif</dt>
          <dd>{branch?.name ?? "Belum dipilih"}</dd>
        </div>
        <div>
          <dt>Terminal aktif</dt>
          <dd>{register?.name ?? "Belum dipilih"}</dd>
        </div>
        <div>
          <dt>Cabang yang dapat diakses</dt>
          <dd>{branches.length}</dd>
        </div>
      </dl>
    </section>
    <section className="panel">
      <div className="panel-heading">
        <h2>Akun Anda</h2>
        <span className="soft-icon"><Icon name="users" /></span>
      </div>
      <dl className="detail-list">
        <div>
          <dt>Email</dt>
          <dd>{user?.email ?? "—"}</dd>
        </div>
        <div>
          <dt>Nomor telepon</dt>
          <dd>{user?.phone ?? "—"}</dd>
        </div>
        <div>
          <dt>Peran</dt>
          <dd>{business?.role ?? "—"}</dd>
        </div>
      </dl>
      <ResourceState resource={profile}>
        {profile.data && <form className="profile-form" key={profile.data.version} onSubmit={saveDisplayName}>
          <Field label="Nama tampilan" id="display_name" required maxLength={200} defaultValue={profile.data.display_name} />
          <button className="primary" disabled={busy}>Simpan nama</button>
        </form>}
      </ResourceState>
      <p className="muted small">Penugasan akun dan cabang ditentukan oleh pemilik bisnis.</p>
    </section>
    <section className="panel form-panel">
      <h2>Ganti kata sandi</h2>
      <Link to="/keamanan">Periksa sesi aktif dan cabut perangkat lain</Link>
      <p className="muted small">Minimal 8 karakter dan bukan kata sandi umum. Setelah berhasil, sesi pada perangkat lain dicabut.</p>
      <form onSubmit={changePassword}><fieldset disabled={busy}>
        <div className="form-grid">
          <Field label="Kata sandi saat ini" id="current_password" name="current_password" type="password" required autoComplete="current-password" />
          <Field label="Kata sandi baru" id="new_password" name="new_password" type="password" minLength={8} required autoComplete="new-password" />
          <Field label="Ulangi kata sandi baru" id="confirm_password" name="confirm_password" type="password" minLength={8} required autoComplete="new-password" />
        </div>
        <button className="primary">Perbarui kata sandi</button>
      </fieldset></form>
    </section>
    {business?.role === "owner" && <section className="panel form-panel">
      <h2>Aturan pajak bisnis</h2>
      <Link to="/pajak">Kelola aturan pajak dan penugasan produk/kategori</Link>
      <p className="muted small">Mengganti aturan aktif untuk seluruh bisnis. Konfirmasikan tarif yang berlaku dengan konsultan pajak Anda.</p>
      <form onSubmit={e => save(e, "/settings/tax", values => ({
          name: values.tax_name.trim(),
          rate_bps: Number(rupiahToCents(values.rate)),
          mode: values.mode
        }))}>
        <div className="form-grid">
          <Field label="Nama pajak" id="tax_name" required maxLength={100} />
          <Field label="Tarif (%)" id="rate" type="number" min="0" max="100" step="0.01" required />
          <Field label="Mode" id="mode"><select id="mode" name="mode">
              <option value="exclusive">Di luar harga</option>
              <option value="inclusive">Termasuk harga</option>
            </select></Field>
        </div>
        <button className="primary" disabled={busy}>Simpan aturan pajak</button>
      </form>
    </section>}
    {["owner", "manager"].includes(business?.role) && <section className="panel form-panel">
      <h2>PIN otorisasi</h2>
      <p className="muted small">Dipakai untuk menyetujui tindakan yang memerlukan otorisasi atasan. Jangan bagikan PIN ini.</p>
      <form onSubmit={e => save(e, "/memberships/me/pin", values => ({
          pin: values.pin
        }))}>
        <Field label="PIN baru (6 digit)" id="pin" type="password" inputMode="numeric" pattern="[0-9]{6}" minLength={6} maxLength={6} required autoComplete="new-password" />
        <button className="primary" disabled={busy}>Simpan PIN</button>
      </form>
    </section>}
  </div>
  <section className="panel form-panel"><h2>Terima undangan bisnis</h2><p className="muted small">Masukkan token yang diberikan Pemilik/Manajer. Email akun Anda harus sesuai dengan email penerima undangan.</p><form onSubmit={acceptInvitation}><Field label="Token undangan" id="invitation_token" required autoComplete="off" /><button className="primary" disabled={busy}>Terima undangan</button></form></section>
  {business?.role === 'owner' && <><OrganizationSettings /><PaymentMethodSettings /></>}
  {business?.business_type === 'restaurant' && ['owner', 'manager'].includes(business?.role) && <KitchenSettings />}
</>;
}
