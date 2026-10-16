import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { PageHeading, Field, Notice } from '../components/UI.jsx';
import { rupiahToCents } from '../forms.js';
import { onboardingEventBody } from '../onboarding.js';
import { Icon } from '../components/Icon.jsx';

const STEPS = [
  ['business', 'Nama bisnis', 'Nama yang digunakan untuk mengenali bisnis Anda di aplikasi.', 'store'],
  ['type', 'Jenis usaha', 'Pilih modul operasional sesuai usaha: retail, restoran, atau apotek.', 'dashboard'],
  ['branch', 'Cabang & terminal', 'Lengkapi lokasi dan zona waktu cabang. Terminal kasir dibuat jika belum tersedia.', 'store'],
  ['settings', 'Pajak, pembulatan & struk', 'Atur perhitungan transaksi dan informasi yang tercetak pada struk.', 'receipt'],
  ['product', 'Produk awal', 'Tambahkan produk pertama beserta SKU, satuan, dan harga jualnya.', 'box'],
];

export default function OnboardingPage() {
  const { business, businessId } = useSession();
  const { branch } = useWorkspace();
  if (business?.role !== 'owner') return <Notice>Panduan pengaturan awal hanya untuk Pemilik.</Notice>;
  return <Wizard key={`${businessId}.${branch?.id ?? 'new'}`} />;
}

function Wizard() {
  const { client, business, muatUlang } = useSession();
  const workspace = useWorkspace();
  const [step, setStep] = useState(0);
  const [busy, setBusy] = useState(true);
  const [message, setMessage] = useState('');
  const [values, setValues] = useState({});
  const [loaded, setLoaded] = useState(false);
  const [finished, setFinished] = useState(false);
  const [saved, setSaved] = useState(false);
  const [reloadRequired, setReloadRequired] = useState(false);
  const [branchId, setBranchId] = useState(workspace.branch?.id ?? '');

  useEffect(() => { load(0); }, []);

  async function load(index) {
    setStep(index); setBusy(true); setMessage(''); setLoaded(false); setSaved(false); setReloadRequired(false);
    try {
      let data;
      if (index === 0) data = await client.get('/settings/business-profile');
      if (index === 1) data = await client.get('/settings/business-type');
      if (index === 2) data = branchId ? await client.get(`/branches/${branchId}`) : { code: 'PUSAT', name: 'Cabang utama', timezone: 'Asia/Jakarta', address: '' };
      if (index === 3) {
        if (!branchId) throw new Error('Buat cabang pada langkah sebelumnya terlebih dahulu.');
        const [branch, policy, tax, type] = await Promise.all([client.get(`/branches/${branchId}`), client.get('/settings/operational-policy'), client.get('/settings/tax'), client.get('/settings/business-type')]);
        const sc = type.business_type === 'restaurant' ? await client.get(`/restaurant/service-charge?branch_id=${branchId}`) : null;
        data = { branch, policy, tax, type, sc }; 
      }
      setValues(data ?? {}); setLoaded(true);
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }

  async function event(status) {
    await client.post('/onboarding/events', onboardingEventBody(STEPS[step][0], status));
  }
  async function next(status = 'completed') {
    setBusy(true); setMessage('');
    try {
      await event(status);
      if (step === STEPS.length - 1) { setFinished(true); workspace.reload(); await muatUlang(); }
      else await load(step + 1);
    } catch (error) { setMessage(`Progres belum tersimpan: ${error.message}. Coba lanjut lagi tanpa menyimpan ulang formulir.`); }
    finally { setBusy(false); }
  }
  async function save(e) {
    e.preventDefault();
    const v = Object.fromEntries(new FormData(e.currentTarget));
    setBusy(true); setMessage('');
    try {
      if (step === 0) await client.put('/settings/business-profile', { name: v.name.trim() }, { version: values.version });
      if (step === 1 && !values.locked && v.business_type !== values.business_type) await client.put('/settings/business-type', { business_type: v.business_type }, { version: values.version });
      if (step === 2) {
        const body = { code: v.code, name: v.name.trim(), timezone: v.timezone, address: v.address, receipt_header: values.receipt_header ?? '', receipt_footer: values.receipt_footer ?? '' };
        const result = branchId ? await client.put(`/branches/${branchId}`, body, { version: values.version }) : await client.post('/branches', body);
        const bid = branchId || result.branch_id;
        setBranchId(bid);
        const registers = await client.get(`/branches/${bid}/registers`);
        if (!registers.registers?.length) await client.post(`/branches/${bid}/registers`, { name: 'Kasir utama' });
      }
      if (step === 3) {
        // Langkah ini memakai beberapa endpoint: jika ada yang gagal, muat ulang nilai/versi sebelum mengulang.
        await client.put('/settings/tax', { name: v.tax_name.trim() || 'Tanpa pajak', rate_bps: Number(rupiahToCents(v.tax_rate)), mode: v.tax_mode });
        await client.put('/settings/operational-policy', { ...values.policy.policy, cash_rounding_enabled: v.rounding === 'on', cash_rounding_multiple_cents: rupiahToCents(v.rounding_multiple) }, { version: values.policy.version });
        if (values.sc) {
          for (const orderType of ['DINE_IN', 'TAKEAWAY', 'DELIVERY_MANUAL']) {
            const rule = values.sc.rules?.find(r => r.order_type === orderType);
            await client.put('/restaurant/service-charge', { branch_id: branchId, order_type: orderType, rate_bps: Number(rupiahToCents(v[`sc_${orderType}`])), include_in_tax_base: v[`sc_tax_${orderType}`] === 'on' }, { version: rule?.version ?? 0 });
          }
        }
        const { branch_id, version, code, name, timezone, address } = values.branch;
        await client.put(`/branches/${branch_id}`, { code, name, timezone, address, receipt_header: v.receipt_header, receipt_footer: v.receipt_footer }, { version });
      }
      if (step === 4) await client.post('/products', { name: v.name.trim(), track_stock: v.track_stock === 'on', variants: [{ sku: v.sku.trim(), base_unit: v.unit.trim(), price_cents: rupiahToCents(v.price), allow_fractional: false }] });
      setSaved(true);
    } catch (error) { setReloadRequired(true); setMessage(`${error.message} Muat ulang langkah sebelum mengulang penyimpanan.`); }
    finally { setBusy(false); }
  }

  if (finished) return <section className="panel setup-finished"><span className="setup-finished-icon"><Icon name="check" size={32} /></span><h1>Panduan pengaturan selesai</h1><p>Langkah yang Anda simpan sudah diterapkan. Lengkapi pengaturan yang dilewati sebelum memulai operasional.</p><div className="row"><Link className="button primary" to="/kasir"><Icon name="cart" size={18} />Buka kasir</Link><Link className="button" to="/anggota">Undang tim</Link><Link className="button" to="/dashboard">Ke dashboard</Link></div></section>;
  return <div className="setup-page">
    <PageHeading title="Panduan pengaturan awal" description="Siapkan identitas usaha, cabang, dan transaksi melalui lima langkah."><Link className="button" to="/dashboard">Kembali ke dashboard</Link></PageHeading>
    <Notice>{message}</Notice>
    <div className="setup-layout">
    <aside className="setup-guide"><div className="setup-guide-heading"><span className="setup-guide-icon"><Icon name="settings" size={24} /></span><div><strong>Siapkan ruang kerja</strong><p>Langkah {step + 1} dari {STEPS.length}</p></div></div>
    <ol className="setup-steps" aria-label="Langkah pengaturan">{STEPS.map(([key, label], i) => <li key={key} aria-current={step === i ? 'step' : undefined}><span className="setup-step-number">{i + 1}</span><div><strong>{label}</strong><span>{step === i ? 'Sedang dibuka' : `Langkah ${i + 1}`}</span></div></li>)}</ol>
    <p className="setup-guide-note">Anda mengatur bisnis yang sudah dimiliki. Data akun dan bisnis baru tidak dibuat melalui panduan ini.</p>
    </aside>
    <section className="panel setup-form" aria-labelledby="setup-step-title" aria-busy={busy}>
      <header className="setup-form-heading"><span className="setup-form-icon"><Icon name={STEPS[step][3]} size={25} /></span><div><span>Langkah {step + 1} / {STEPS.length}</span><h2 id="setup-step-title">{STEPS[step][1]}</h2></div></header>
      <p className="setup-description">{STEPS[step][2]}</p>
      {!loaded ? busy ? <div className="empty-state" role="status"><span className="loading-ring" />Memuat pengaturan…</div> : <div className="empty-state"><Icon name="refresh" size={30} /><h3>Pengaturan belum dapat dimuat</h3><p>Coba muat kembali untuk melanjutkan langkah ini.</p><button onClick={() => load(step)}>Coba lagi</button></div> : saved ? <div className="setup-saved"><span className="setup-finished-icon"><Icon name="check" size={26} /></span><h3>Pengaturan langkah ini tersimpan</h3><p>Anda dapat melanjutkan ke {step === STEPS.length - 1 ? 'akhir panduan' : `langkah ${STEPS[step + 1][1].toLowerCase()}`}.</p><button className="primary" disabled={busy} onClick={() => next()}>{busy ? 'Menyimpan progres…' : step === STEPS.length - 1 ? 'Selesaikan panduan' : 'Lanjut ke langkah berikutnya'}<Icon name="arrow" size={17} /></button></div> : <form key={`${step}-${JSON.stringify(values)}`} onSubmit={save}><fieldset disabled={busy || reloadRequired}><div className={`setup-fields setup-fields-step-${step}`}>
        {step === 0 && <Field label="Nama bisnis" id="name" required maxLength={200} defaultValue={values.name} />}
        {step === 1 && <><p>Jenis usaha terkunci setelah penjualan pertama.</p><Field label="Jenis usaha" id="business_type"><select name="business_type" id="business_type" defaultValue={values.business_type} disabled={values.locked}><option value="retail">Retail</option><option value="restaurant">Restoran</option><option value="pharmacy">Apotek</option></select></Field></>}
        {step === 2 && <><Field label="Kode cabang" id="code" required pattern="[A-Z0-9-]+" maxLength={20} defaultValue={values.code} readOnly={!!branchId} /><Field label="Nama cabang" id="name" required maxLength={200} defaultValue={values.name} /><Field label="Alamat" id="address" maxLength={1000} defaultValue={values.address} /><Field label="Zona waktu IANA" id="timezone" required defaultValue={values.timezone} /><p>Terminal Kasir utama dibuat hanya jika cabang belum punya terminal aktif.</p></>}
        {step === 3 && <><Field label="Nama pajak" id="tax_name" maxLength={100} defaultValue={values.tax.rule?.name ?? 'Tanpa pajak'} /><Field label="Pajak (%; 0 = nonaktif)" id="tax_rate" type="number" min="0" max="100" step="0.01" required defaultValue={(values.tax.rule?.rate_bps ?? 0) / 100} /><Field label="Mode pajak" id="tax_mode"><select name="tax_mode" id="tax_mode" defaultValue={values.tax.rule?.mode ?? 'exclusive'}><option value="exclusive">Eksklusif</option><option value="inclusive">Inklusif</option></select></Field><label><input name="rounding" type="checkbox" defaultChecked={values.policy.policy.cash_rounding_enabled} /> Pembulatan tunai aktif</label><Field label="Kelipatan pembulatan (Rp)" id="rounding_multiple" type="number" min="1" max="1000" step="1" required defaultValue={Number(values.policy.policy.cash_rounding_multiple_cents) / 100} />{values.sc && ['DINE_IN', 'TAKEAWAY', 'DELIVERY_MANUAL'].map(orderType => { const rule = values.sc.rules?.find(r => r.order_type === orderType); return <div key={orderType}><Field label={`Service charge ${orderType} (%)`} id={`sc_${orderType}`} type="number" min="0" max="100" step="0.01" required defaultValue={(rule?.rate_bps ?? 0) / 100} /><label><input name={`sc_tax_${orderType}`} type="checkbox" defaultChecked={rule?.include_in_tax_base ?? true} /> Service charge masuk dasar pajak</label></div>; })}<Field label="Header struk" id="receipt_header" maxLength={1000} defaultValue={values.branch.receipt_header} /><Field label="Footer struk" id="receipt_footer" maxLength={1000} defaultValue={values.branch.receipt_footer} /></>}
        {step === 4 && <><Field label="Nama produk" id="name" required maxLength={200} /><Field label="SKU" id="sku" required maxLength={100} /><Field label="Satuan dasar" id="unit" required defaultValue="pcs" /><Field label="Harga jual (Rp)" id="price" type="number" min="0" step="0.01" required /><label><input name="track_stock" type="checkbox" defaultChecked /> Lacak stok (stok awal dicatat lewat penerimaan)</label></>}
        </div><div className="setup-form-actions"><button className="primary">{busy ? 'Menyimpan…' : 'Simpan pengaturan'}<Icon name="check" size={17} /></button><button type="button" disabled={step === 0} onClick={() => load(step - 1)}>Kembali</button><button className="setup-skip" type="button" onClick={() => next('skipped')}>Lewati langkah</button></div>
      </fieldset></form>}
      {loaded && !saved && <div className="setup-reload"><span className="muted small">{reloadRequired ? 'Muat ulang sebelum menyimpan kembali.' : 'Ingin memakai data terbaru dari server?'}</span><button disabled={busy} onClick={() => load(step)}><Icon name="refresh" size={16} />Muat ulang nilai</button></div>}
      {step === 4 && <p className="setup-note">Sudah memiliki daftar produk? <Link to="/produk">Tambah melalui katalog</Link>{business?.business_type === 'retail' && <> atau <Link to="/impor-katalog">impor CSV</Link></>}, atau gunakan data contoh di <Link to="/dashboard">dashboard</Link>.</p>}
      {business?.business_type === 'restaurant' && <p className="setup-note">Sebelum pesanan pertama, atur service charge di <Link to="/pengaturan">pengaturan restoran</Link> dan pastikan kategori menu sudah terhubung ke stasiun dapur aktif.</p>}
    </section>
    </div>
  </div>;
}
