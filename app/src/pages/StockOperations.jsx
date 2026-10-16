import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { usePendingMutation } from '../hooks/usePendingMutation.js';
import { newIdempotencyKey } from '../api/client.js';
import { quantityToMilli, optionalApproval } from '../cashier.js';
import { formatQty } from '../money.js';
import { rupiahToCents } from '../forms.js';
import { resolveDefaultUnit } from '../units.js';
import { localDate } from '../reporting.js';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { Notice, ResourceState } from '../components/UI.jsx';

const reasonLabels = {
  damaged: 'Rusak',
  lost: 'Hilang',
  expired: 'Kedaluwarsa',
  own_use: 'Dipakai sendiri',
  correction: 'Koreksi pencatatan',
  other: 'Lainnya',
};

export default function StockOperations({ onChanged }) {
  const { businessId, user } = useSession();
  const { branch } = useWorkspace();
  return <StockWorkspace key={`${businessId}.${user?.id}.${branch?.id}`} onChanged={onChanged} />;
}

function StockWorkspace({ onChanged }) {
  const { client, business, businessId, user } = useSession();
  const { branch } = useWorkspace();
  const receiptPost = usePendingMutation(`storepos.pendingReceiptPost.${businessId}.${user?.id}.${branch?.id}`);
  const canAdjust = ['owner', 'manager', 'warehouse', 'pharmacist'].includes(business?.role);
  const canCount = canAdjust;
  const canSetMinimum = ['owner', 'manager', 'pharmacist'].includes(business?.role);
  const canReceive = canAdjust;
  const views = [...(canReceive ? [['receipt', 'Penerimaan']] : []), ...(canAdjust ? [['adjustment', 'Penyesuaian']] : []), ['card', 'Kartu stok'], ...(canSetMinimum ? [['minimum', 'Stok minimum']] : []), ...(canCount ? [['count', 'Opname']] : [])];
  const [view, setView] = useState(canAdjust ? 'adjustment' : 'card');
  const [variantID, setVariantID] = useState('');
  const [cardCursor, setCardCursor] = useState('');
  const [count, setCount] = useState(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');
  const [pending, setPending] = useState(null);
  const [receiptLines, setReceiptLines] = useState([{ variant_id: '', unit_id: '', qty: '', price: '' }]);
  const [units, setUnits] = useState({});
  const [receiptDraft, setReceiptDraft] = useState(null);
  const adjustmentForm = useRef(null);
  const pendingKey = `storepos.stockAdjustment.pending.${businessId}.${user?.id}.${branch?.id ?? 'branch'}`;
  const catalog = useResource('/catalog/search?limit=100');
  const suppliers = useResource(canReceive ? '/suppliers' : null);
  const reorder = useResource(branch ? `/stock/reorder?branch_id=${encodeURIComponent(branch.id)}` : null);
  const cardPath = branch && variantID && view === 'card'
    ? `/stock/card?branch_id=${encodeURIComponent(branch.id)}&variant_id=${encodeURIComponent(variantID)}${cardCursor ? `&before=${encodeURIComponent(cardCursor)}` : ''}`
    : null;
  const card = useResource(cardPath);
  const products = (catalog.data?.items ?? []).filter(item => item.track_stock);
  const productByID = useMemo(() => new Map(products.map(item => [item.variant.id, item])), [products]);
  const canPostCount = ['owner', 'manager'].includes(business?.role);

  useEffect(() => {
    try {
      setPending(JSON.parse(sessionStorage.getItem(pendingKey)) || null);
    } catch {
      setPending(null);
    }
  }, [pendingKey]);

  function selectView(next) {
    setView(next);
    setMessage('');
    setSuccess('');
    if (next !== 'card') setCardCursor('');
  }

  function adjust(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setMessage('');
    setSuccess('');
    try {
      let qty = quantityToMilli(form.get('qty'));
      if (form.get('direction') === 'out') qty = `-${qty}`;
      const approval = optionalApproval(form.get('approval_email'), form.get('approval_pin'));
      const body = {
        branch_id: branch.id,
        variant_id: form.get('variant_id'),
        qty_milli: qty,
        reason: form.get('reason'),
        note: String(form.get('note')).trim(),
        ...(approval ? { approval } : {}),
      };
      const operation = { body, key: newIdempotencyKey() };
      sessionStorage.setItem(pendingKey, JSON.stringify(operation));
      setPending(operation);
      sendAdjustment(operation);
    } catch (error) {
      setMessage(error.message);
    }
  }

  async function sendAdjustment(operation) {
    if (busy) return;
    setBusy(true);
    setMessage('');
    setSuccess('');
    try {
      const result = await client.post('/stock/adjustments', operation.body, { idempotencyKey: operation.key });
      sessionStorage.removeItem(pendingKey);
      setPending(null);
      adjustmentForm.current?.reset();
      setSuccess(`Penyesuaian disimpan. Saldo terbaru ${formatQty(BigInt(result.balance_after_milli))}.`);
      reorder.reload();
      onChanged?.();
    } catch (error) {
      setMessage(error.message);
      if (error.status && error.status < 500) {
        sessionStorage.removeItem(pendingKey);
        setPending(null);
      }
    } finally {
      setBusy(false);
    }
  }

  async function setMinimum(event, item) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setMessage('');
    setSuccess('');
    try {
      await client.put(`/stock/minimums/${item.variant_id}`, {
        branch_id: branch.id,
        minimum_qty_milli: quantityToMilli(form.get('minimum'), true),
      }, { version: item.version });
      setSuccess(`Stok minimum ${item.product_name} diperbarui.`);
      reorder.reload();
      onChanged?.();
    } catch (error) {
      setMessage(error.status === 409 ? 'Stok minimum berubah. Muat ulang daftar lalu coba kembali.' : error.message);
    } finally {
      setBusy(false);
    }
  }

  async function createMinimum(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setMessage('');
    setSuccess('');
    try {
      await client.put(`/stock/minimums/${form.get('variant_id')}`, {
        branch_id: branch.id,
        minimum_qty_milli: quantityToMilli(form.get('minimum'), true),
      }, { version: 0 });
      event.currentTarget.reset();
      setSuccess('Stok minimum berhasil dibuat.');
      reorder.reload();
      onChanged?.();
    } catch (error) {
      setMessage(error.status === 409 ? 'Produk ini sudah memiliki stok minimum. Ubah melalui daftar kebutuhan pengadaan.' : error.message);
    } finally {
      setBusy(false);
    }
  }

  async function startCount() {
    setBusy(true);
    setMessage('');
    setSuccess('');
    try {
      const started = await client.post('/stock/counts', { branch_id: branch.id });
      setCount(await client.get(`/stock/counts/${started.count_id}`));
      setSuccess('Sesi opname dibuat dari snapshot stok saat ini.');
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }

  async function recordCount(event, line) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setMessage('');
    setSuccess('');
    try {
      const result = await client.put(`/stock/counts/${count.count_id}/lines/${line.variant_id}`, {
        counted_qty_milli: quantityToMilli(form.get('qty'), true),
      }, { version: count.version });
      const latest = await client.get(`/stock/counts/${count.count_id}`);
      setCount({ ...latest, version: result.version });
      setSuccess('Jumlah fisik disimpan.');
    } catch (error) {
      setMessage(error.status === 409 ? 'Opname diubah pengguna lain. Muat ulang sesi sebelum melanjutkan.' : error.message);
    } finally {
      setBusy(false);
    }
  }

  async function postCount() {
    setBusy(true);
    setMessage('');
    setSuccess('');
    try {
      const result = await client.post(`/stock/counts/${count.count_id}/post`, {}, { version: count.version });
      setCount(current => ({ ...current, status: result.status, version: result.version }));
      setSuccess('Opname diposting dan saldo stok telah disesuaikan.');
      reorder.reload();
      onChanged?.();
    } catch (error) {
      setMessage(error.status === 409 ? 'Opname tidak dapat diposting karena versi atau saldo berubah. Muat ulang sesi.' : error.message);
    } finally {
      setBusy(false);
    }
  }

  async function selectReceiptProduct(index, variantID) {
    setReceiptLines(current => current.map((line, i) => i === index ? { ...line, variant_id: variantID, unit_id: '' } : line));
    if (!variantID) return;
    try {
      const [data, defaults] = await Promise.all([client.get(`/variants/${variantID}/units`), client.get(`/variants/${variantID}/unit-defaults`)]);
      const unit = resolveDefaultUnit(data.units ?? [], defaults, 'purchase');
      setUnits(current => ({ ...current, [variantID]: data.units ?? [] }));
      setReceiptLines(current => current.map((line, i) => i === index && line.variant_id === variantID ? { ...line, unit_id: unit?.unit_id ?? '' } : line));
    } catch (error) {
      setMessage(error.message);
    }
  }

  function updateReceiptLine(index, patch) {
    setReceiptLines(current => current.map((line, i) => i === index ? { ...line, ...patch } : line));
  }

  async function createReceipt(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setMessage('');
    setSuccess('');
    try {
      const result = await receiptPost.send({ path: '/goods-receipts', key: newIdempotencyKey(), body: {
        branch_id: branch.id,
        supplier_id: form.get('supplier_id'),
        invoice_number: String(form.get('invoice_number')).trim(),
        received_date: form.get('received_date'),
        lines: receiptLines.map(line => ({
          unit_id: line.unit_id,
          qty_milli: quantityToMilli(line.qty),
          purchase_price_cents: rupiahToCents(line.price),
        })),
      } });
      setReceiptDraft(result);
      setSuccess('Draft penerimaan dibuat. Periksa data lalu posting untuk menambah stok.');
    } catch (error) {
      setMessage(!error.status ? 'Status pembuatan draft belum pasti. Periksa penerimaan di server sebelum mengirim ulang.' : error.message);
    } finally {
      setBusy(false);
    }
  }

  async function postReceipt(operation = null) {
    if (busy) return;
    setBusy(true);
    setMessage('');
    setSuccess('');
    try {
      const result = await receiptPost.send(operation ?? { path: `/goods-receipts/${receiptDraft.receipt_id}/post`, body: {}, version: receiptDraft.version, key: newIdempotencyKey() });
      setReceiptDraft(result);
      setSuccess(result.status === 'draft' ? 'Draft penerimaan ditemukan. Periksa lalu posting.' : 'Penerimaan diposting dan stok berhasil ditambahkan.');
      reorder.reload();
      onChanged?.();
    } catch (error) {
      setMessage(!error.status ? 'Status posting belum pasti. Jangan posting ulang sebelum memeriksa status penerimaan.' : error.status === 409 ? 'Draft penerimaan berubah atau sudah diposting.' : error.message);
    } finally {
      setBusy(false);
    }
  }

  return <section className="panel stock-operations">
    <div className="panel-heading"><div><h2>Operasi persediaan</h2><p className="muted">Catat perubahan stok dan telusuri pergerakan pada cabang aktif.</p></div></div>
    <div className="operation-tabs" role="tablist" aria-label="Operasi persediaan">
      {views.map(([key, label]) => <button key={key} role="tab" aria-selected={view === key} className={view === key ? 'primary' : ''} onClick={() => selectView(key)}>{label}</button>)}
    </div>
    <Notice>{message || receiptPost.error}</Notice><Notice kind="success">{success}</Notice>
    {receiptPost.pending && <div className="alert warn" role="status">Status pembuatan/posting penerimaan belum pasti.<button disabled={busy || receiptPost.sending} onClick={() => postReceipt(receiptPost.pending)}>Periksa & coba lagi dengan key asli</button><Link to="/penerimaan">Buka dokumen penerimaan</Link></div>}
    {pending && <div className="alert warn" role="status"><p>{busy ? 'Mencatat penyesuaian…' : 'Status penyesuaian belum pasti. Periksa dengan kunci yang sama agar stok tidak berubah dua kali.'}</p><button disabled={busy} onClick={() => sendAdjustment(pending)}>Periksa & coba lagi</button></div>}
    {!branch ? <div className="empty-state">Pilih cabang untuk menjalankan operasi stok.</div> : <ResourceState resource={catalog} empty={!products.length} message="Tambahkan produk yang melacak stok sebelum menjalankan operasi persediaan.">
      {view === 'receipt' && <div className="stock-stack">
        <Link to="/penerimaan">Cari, muat, edit draft, atau periksa status penerimaan</Link>
        {receiptDraft?.status === 'posted' ? <div className="empty-state"><h3>Penerimaan selesai</h3><p>Dokumen {receiptDraft.receipt_id} sudah diposting ke stok.</p><button onClick={() => { setReceiptDraft(null); setReceiptLines([{ variant_id: '', unit_id: '', qty: '', price: '' }]); }}>Penerimaan baru</button></div> : <form className="stock-form" onSubmit={createReceipt}>
          <fieldset disabled={busy || !!receiptDraft || receiptPost.blocked}><div className="form-grid">
            <label className="field">Supplier<select name="supplier_id"><option value="">Tanpa supplier</option>{suppliers.data?.suppliers?.filter(item => !item.archived).map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
            <label className="field">Nomor invoice<input name="invoice_number" maxLength="200" /></label>
            <label className="field">Tanggal diterima<input name="received_date" type="date" defaultValue={localDate(branch.timezone)} required /></label>
          </div>
          <div className="receipt-lines"><div className="receipt-line-head"><strong>Barang diterima</strong><button type="button" onClick={() => setReceiptLines(current => [...current, { variant_id: '', unit_id: '', qty: '', price: '' }])}>Tambah baris</button></div>
            {receiptLines.map((line, index) => <div className="receipt-line" key={index}>
              <label>Produk<select required value={line.variant_id} onChange={event => selectReceiptProduct(index, event.target.value)}><option value="">Pilih produk</option>{products.map(item => <option key={item.variant.id} value={item.variant.id}>{item.product_name} · {item.variant.sku}</option>)}</select></label>
              <label>Satuan beli<select required value={line.unit_id} onChange={event => updateReceiptLine(index, { unit_id: event.target.value })}><option value="">Pilih satuan</option>{(units[line.variant_id] ?? []).map(unit => <option key={unit.unit_id} value={unit.unit_id}>{unit.name} ({formatQty(BigInt(unit.factor_milli))} satuan dasar)</option>)}</select></label>
              <label>Jumlah<input required type="number" min="0.001" step="0.001" value={line.qty} onChange={event => updateReceiptLine(index, { qty: event.target.value })} /></label>
              <label>Harga per satuan (Rp)<input required type="number" min="0" step="0.01" value={line.price} onChange={event => updateReceiptLine(index, { price: event.target.value })} /></label>
              <button type="button" className="danger" disabled={receiptLines.length === 1} onClick={() => setReceiptLines(current => current.filter((_, i) => i !== index))}>Hapus</button>
            </div>)}
          </div><button className="primary">Buat draft penerimaan</button></fieldset>
        </form>}
        {receiptDraft?.status === 'draft' && <div className="receipt-draft"><div><h3>Draft siap diposting</h3><p className="muted">ID {receiptDraft.receipt_id} · versi {receiptDraft.version}. Posting akan menambah stok dan mengunci faktor satuan.</p></div><button className="primary" onClick={() => postReceipt()} disabled={busy || receiptPost.blocked}>{busy ? 'Memposting…' : 'Posting penerimaan'}</button></div>}
      </div>}

      {view === 'adjustment' && <form ref={adjustmentForm} className="stock-form" onSubmit={adjust}>
        <fieldset disabled={busy || !!pending}><div className="form-grid">
          <label className="field">Produk<select name="variant_id" required><option value="">Pilih produk</option>{products.map(item => <option key={item.variant.id} value={item.variant.id}>{item.product_name} · {item.variant.sku}</option>)}</select></label>
          <label className="field">Perubahan<select name="direction"><option value="in">Tambah stok</option><option value="out">Kurangi stok</option></select></label>
          <label className="field">Jumlah<input name="qty" type="number" min="0.001" step="0.001" required /></label>
          <label className="field">Alasan<select name="reason"><option value="correction">Koreksi pencatatan</option><option value="damaged">Rusak</option><option value="lost">Hilang</option><option value="expired">Kedaluwarsa</option><option value="own_use">Dipakai sendiri</option><option value="other">Lainnya</option></select></label>
          <label className="field field-wide">Catatan<input name="note" maxLength="200" placeholder="Wajib untuk alasan lainnya" /></label>
          <label className="field">Email atasan (bila diminta)<input name="approval_email" type="email" autoComplete="username" /></label>
          <label className="field">PIN atasan (bila diminta)<input name="approval_pin" type="password" inputMode="numeric" pattern="[0-9]{6}" maxLength="6" autoComplete="off" /></label>
        </div><button className="primary">{busy ? 'Menyimpan…' : 'Simpan penyesuaian'}</button></fieldset>
      </form>}

      {view === 'card' && <div>
        <label className="field stock-selector">Produk<select value={variantID} onChange={event => { setVariantID(event.target.value); setCardCursor(''); }}><option value="">Pilih produk</option>{products.map(item => <option key={item.variant.id} value={item.variant.id}>{item.product_name} · {item.variant.sku}</option>)}</select></label>
        {!variantID ? <div className="empty-state">Pilih produk untuk melihat riwayat stok.</div> : <ResourceState resource={card} empty={!card.data?.movements?.length} message="Produk ini belum memiliki pergerakan stok.">
          <div className="table-scroll"><table><thead><tr><th>Waktu</th><th>Jenis</th><th>Alasan</th><th className="num">Perubahan</th><th className="num">Saldo</th></tr></thead><tbody>{card.data?.movements?.map(row => <tr key={row.id}><td>{new Date(row.created_at).toLocaleString('id-ID')}</td><td>{row.kind.replaceAll('_', ' ')}</td><td>{reasonLabels[row.reason] ?? row.reason ?? row.ref_type}{row.note ? ` · ${row.note}` : ''}</td><td className="num">{formatQty(BigInt(row.qty_milli))}</td><td className="num">{formatQty(BigInt(row.balance_after_milli))}</td></tr>)}</tbody></table></div>
          {card.data?.next_cursor && <div className="table-footer"><button onClick={() => setCardCursor(card.data.next_cursor)}>Muat pergerakan lebih lama</button></div>}
        </ResourceState>}
      </div>}

      {view === 'minimum' && <div className="stock-stack">
        <form className="stock-form compact" onSubmit={createMinimum}><h3>Buat stok minimum</h3><div className="form-grid"><label className="field">Produk<select name="variant_id" required><option value="">Pilih produk</option>{products.map(item => <option key={item.variant.id} value={item.variant.id}>{item.product_name} · {item.variant.sku}</option>)}</select></label><label className="field">Jumlah minimum<input name="minimum" type="number" min="0" step="0.001" required /></label></div><button className="primary" disabled={busy}>Simpan minimum</button></form>
        <div><h3>Kebutuhan pengadaan</h3><p className="muted small">Produk dengan saldo sama atau di bawah stok minimum.</p><ResourceState resource={reorder} empty={!reorder.data?.items?.length} message="Tidak ada produk yang perlu dipesan."><div className="table-scroll"><table><thead><tr><th>Produk</th><th>Supplier terakhir</th><th className="num">Saldo</th><th>Ubah minimum</th></tr></thead><tbody>{reorder.data?.items?.map(item => <tr key={item.variant_id}><td><strong>{item.product_name}</strong><small className="table-subtext">{item.sku} · {item.base_unit}</small></td><td>{item.last_supplier_name ?? '—'}</td><td className="num">{formatQty(BigInt(item.qty_milli))} / {formatQty(BigInt(item.minimum_qty_milli))}</td><td><form className="price-form" onSubmit={event => setMinimum(event, item)}><input aria-label={`Minimum ${item.product_name}`} name="minimum" type="number" min="0" step="0.001" defaultValue={formatQty(BigInt(item.minimum_qty_milli)).replace(',', '.')} required /><button disabled={busy}>Simpan</button></form></td></tr>)}</tbody></table></div></ResourceState></div>
      </div>}

      {view === 'count' && <div className="stock-stack">
        {!count && <div className="empty-state"><h3>Mulai opname baru</h3><p>Saldo saat ini akan disalin sebagai snapshot. Produk yang tidak dihitung tidak akan berubah.</p><button className="primary" onClick={startCount} disabled={busy}>{busy ? 'Membuat…' : 'Mulai opname'}</button></div>}
        {count && <><div className="count-heading"><div><h3>Opname {count.status === 'posted' ? 'selesai' : 'berjalan'}</h3><p className="muted">{count.lines.length} produk dalam snapshot · versi {count.version}</p></div>{count.status === 'draft' && canPostCount && <button className="primary" onClick={postCount} disabled={busy}>Posting opname</button>}</div><div className="table-scroll"><table><thead><tr><th>Produk</th><th className="num">Snapshot</th><th className="num">Sistem saat hitung</th><th>Jumlah fisik</th></tr></thead><tbody>{count.lines.map(line => { const item = productByID.get(line.variant_id); return <tr key={line.variant_id}><td>{item ? `${item.product_name} · ${item.variant.sku}` : line.variant_id}</td><td className="num">{formatQty(BigInt(line.snapshot_qty_milli))}</td><td className="num">{line.system_qty_at_count == null ? '—' : formatQty(BigInt(line.system_qty_at_count))}</td><td>{count.status === 'draft' ? <form className="price-form" onSubmit={event => recordCount(event, line)}><input aria-label={`Jumlah fisik ${item?.product_name ?? line.variant_id}`} name="qty" type="number" min="0" step="0.001" defaultValue={line.counted_qty_milli == null ? '' : formatQty(BigInt(line.counted_qty_milli)).replace(',', '.')} required /><button disabled={busy}>Catat</button></form> : line.counted_qty_milli == null ? 'Tidak dihitung' : formatQty(BigInt(line.counted_qty_milli))}</td></tr>; })}</tbody></table></div></>}
      </div>}
    </ResourceState>}
  </section>;
}
