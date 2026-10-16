import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { usePendingMutation } from '../hooks/usePendingMutation.js';
import { newIdempotencyKey } from '../api/client.js';
import { PageHeading, Notice } from '../components/UI.jsx';

export default function CatalogImportPage() {
  const { business, businessId, user } = useSession();
  const { branch } = useWorkspace();
  if (business?.business_type !== 'retail') return <Notice>Impor CSV ini hanya untuk retail; apotek memerlukan batch/ED dan restoran memerlukan konfigurasi menu tersendiri.</Notice>;
  if (!['owner', 'manager'].includes(business?.role)) return <Notice>Impor katalog hanya untuk Pemilik/Manajer.</Notice>;
  if (!branch) return <Notice>Pilih cabang aktif untuk stok awal.</Notice>;
  return <Import key={`${businessId}.${user?.id}.${branch.id}`} branch={branch} />;
}

function Import({ branch }) {
  const { client, businessId, user } = useSession();
  const pending = usePendingMutation(`storepos.pendingCatalogImport.${businessId}.${user?.id}.${branch.id}`);
  const [file, setFile] = useState(null);
  const [raw, setRaw] = useState('');
  const [preview, setPreview] = useState(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [result, setResult] = useState(null);
  const lock = useRef(false);
  const generation = useRef(0);
  const input = useRef(null);
  const request = useRef(null);
  useEffect(() => () => { generation.current += 1; request.current?.abort(); }, []);

  async function selectFile(event) {
    if (pending.blocked || lock.current) return;
    const selected = event.target.files?.[0]; const token = ++generation.current;
    request.current?.abort(); setFile(null); setRaw(''); setPreview(null); setResult(null); setMessage('');
    if (!selected) return;
    if (selected.size > 5 * 1024 * 1024) { setMessage('Berkas maksimal 5 MiB.'); event.target.value = ''; return; }
    try {
      const bytes = await selected.arrayBuffer();
      const text = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
      if (token !== generation.current) return;
      if (!text.trim()) throw new Error('Berkas CSV kosong.');
      setFile({ name: selected.name, size: selected.size }); setRaw(text);
    } catch (error) { if (token === generation.current) { setMessage(`Berkas harus UTF-8 yang valid: ${error.message}`); if (input.current) input.current.value = ''; } }
  }
  async function check() {
    if (lock.current || pending.blocked || !raw) return;
    lock.current = true; setBusy(true); setMessage(''); setPreview(null);
    const controller = new AbortController(); request.current = controller;
    try {
      const data = await client.request('POST', `/catalog/import/preview?branch_id=${branch.id}`, { rawBody: raw, signal: controller.signal });
      if (!controller.signal.aborted) setPreview(data);
    } catch (error) { if (error.name !== 'AbortError') setMessage(error.message); }
    finally { if (request.current === controller) request.current = null; lock.current = false; setBusy(false); }
  }
  async function send(operation) {
    if (lock.current) return;
    lock.current = true; setBusy(true); setMessage('');
    try {
      const data = await pending.send(operation);
      setResult(data); setPreview(null); setRaw(''); setFile(null); if (input.current) input.current.value = '';
    } catch (error) { setMessage(error.message); }
    finally { lock.current = false; setBusy(false); }
  }
  function commit(event) {
    event.preventDefault();
    if (!preview?.valid || busy || pending.blocked || !raw) return;
    send({ path: `/catalog/import/commit?branch_id=${branch.id}`, rawBody: raw, key: newIdempotencyKey() });
  }
  async function template() {
    if (lock.current) return;
    lock.current = true; setBusy(true); setMessage('');
    let url;
    try {
      const blob = await client.get('/catalog/import/template', { responseType: 'blob' });
      url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href = url; a.download = 'storepos-catalog-template.csv'; a.click();
    } catch (error) { setMessage(error.message); }
    finally { if (url) setTimeout(() => URL.revokeObjectURL(url), 1000); lock.current = false; setBusy(false); }
  }
  return <>
    <PageHeading title="Impor katalog CSV" description={`Cabang stok awal: ${branch.name}. Satu baris membuat satu produk dan varian baru; tidak menimpa produk yang sudah ada.`}><Link to="/produk">Katalog</Link></PageHeading>
    <Notice>{message || pending.error}</Notice>
    {pending.pending && <div className="alert warn" role="alert">Hasil impor belum pasti. Jangan unggah ulang atau membuat key baru. File asli disimpan pada tab ini sampai hasil pasti.<button disabled={busy || pending.sending} onClick={() => send(pending.pending)}>Periksa & coba lagi dengan file/key asli</button></div>}
    {result && <Notice kind="success">{result.imported_rows} baris berhasil diimpor.</Notice>}
    <section className="panel"><h2>1. Unduh template dan isi berkas</h2><button disabled={busy || pending.blocked} onClick={template}>Unduh template CSV</button>
      <p>UTF-8, maksimal 5 MiB dan 5.000 baris. Header dan urutan kolom harus sama dengan template.</p>
      <p>Harga/HPP dalam <strong>integer sen</strong>, jumlah dalam <strong>milli-unit</strong> (1 unit = 1000). Boolean true/false. Stok awal positif wajib HPP; barcode opsional. Kategori, penggabungan multivarian dan satuan kemasan tambahan belum diimpor.</p>
      <p className="muted">Contoh Rp10.000 = 1000000 sen; 2 barang = 2000 milli-unit. Jangan isi harga dengan pemisah ribuan. Isi HPP per satuan dasar, bukan total HPP.</p>
      <label htmlFor="catalog_csv">2. Pilih berkas<input ref={input} id="catalog_csv" type="file" accept=".csv,text/csv" disabled={busy || pending.blocked} onChange={selectFile} /></label>
      {file && <p role="status">{file.name} · {file.size} byte</p>}
      <button disabled={busy || pending.blocked || !raw} onClick={check}>{busy ? 'Memproses…' : '3. Pratinjau validasi server'}</button>
    </section>
    {preview && <section className="panel"><h2>Hasil pratinjau</h2><p role="status">{preview.row_count} baris · {preview.valid ? 'Valid' : 'Perlu diperbaiki'}</p>
      {preview.errors?.length > 0 && <div className="table-scroll"><table><thead><tr><th>Baris CSV</th><th>Kolom</th><th>Pesan</th></tr></thead><tbody>{preview.errors.map((error,index) => <tr key={`${error.row}-${error.field}-${index}`}><td>{error.row}</td><td>{error.field}</td><td>{error.message}</td></tr>)}</tbody></table></div>}
      <p>Pratinjau tidak menyimpan data. Server memvalidasi kembali ketika commit; bila satu baris gagal, seluruh impor dibatalkan.</p>
      {preview.valid && <form key={`${file?.name}-${file?.size}`} onSubmit={commit}><fieldset disabled={busy || pending.blocked}><label><input type="checkbox" required /> Saya sudah memeriksa cabang, harga, jumlah dan HPP. Commit akan membuat produk serta stok awal.</label><button className="primary">4. Impor seluruh file secara atomik</button></fieldset></form>}
    </section>}
  </>;
}
