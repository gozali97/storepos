import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { usePendingMutation } from '../hooks/usePendingMutation.js';
import { Link, useParams } from 'react-router-dom';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { newIdempotencyKey } from '../api/client.js';
import { formatIDR, formatQty } from '../money.js';
import { addLineBody, cancelBody, cancelNeedsApproval, splitCheckoutPayments, editLineBody, orderDiscountBody, lineStatusLabel, liveLines, modifierError, notSentCount, paymentBlockReason } from '../orders.js';
import { PageHeading, Field, Notice } from '../components/UI.jsx';
import { Icon } from '../components/Icon.jsx';
import { RestaurantOrderHeader } from '../components/RestaurantOrderHeader.jsx';

function OrderActionModal({ title, busy, error, onClose, children }) {
  const dialog = useRef(null);
  useEffect(() => {
    const trigger = document.activeElement;
    dialog.current.showModal();
    return () => { if (trigger?.isConnected) trigger.focus(); };
  }, []);
  return createPortal(<dialog ref={dialog} className="order-action-modal" aria-labelledby="order-action-title" onCancel={event => {
    event.preventDefault();
    if (!busy) onClose();
  }}>
    <div className="panel-heading"><h2 id="order-action-title">{title}</h2><button className="icon-button" type="button" aria-label="Tutup dialog" disabled={busy} onClick={onClose}><Icon name="close" /></button></div>
    <Notice>{error}</Notice>
    {children}
  </dialog>, document.body);
}

export default function OrderPage() {
  const { id } = useParams();
  const { businessId, user } = useSession();
  return <OrderWorkspace key={`${businessId}.${user?.id}.${id}`} id={id} />;
}

function OrderWorkspace({ id }) {
  const { client, business, businessId, user } = useSession();
  const { register } = useWorkspace();
  const methodSettings = useResource('/settings/payment-methods');
  const configuredMethods = methodSettings.data?.methods ?? [];
  const order = useResource(`/restaurant/orders/${id}/lines`);
  const header = useResource(`/restaurant/orders/${id}`);
  const bill = useResource(order.data ? `/restaurant/orders/${id}/bill?v=${order.data.version}` : null);
  const [query, setQuery] = useState('');
  const [hits, setHits] = useState([]);
  const [picked, setPicked] = useState(null);
  const [groups, setGroups] = useState([]);
  const [chosen, setChosen] = useState([]);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');
  const [paid, setPaid] = useState(null);
  const [cancelling, setCancelling] = useState(null);
  const [editing, setEditing] = useState(null);
  const [deleting, setDeleting] = useState(null);
  const [discountTarget, setDiscountTarget] = useState(null);
  const [payments, setPayments] = useState([{ id: newIdempotencyKey(), method: 'cash', amount: '' }]);
  const mutation = usePendingMutation(`storepos.pendingRestaurant.${businessId}.${user?.id}.${id}`);

  const version = order.data?.version;
  const lines = order.data?.lines ?? [];
  const pending = notSentCount(lines);
  const blocked = paymentBlockReason(bill.data);

  useEffect(() => {
    if (!query.trim()) { setHits([]); return undefined; }
    const controller = new AbortController();
    const timer = setTimeout(() => {
      client.get(`/catalog/search?q=${encodeURIComponent(query.trim())}&limit=20`, { signal: controller.signal })
        .then(data => setHits(data.items ?? []), () => {});
    }, 250);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [query, client]);

  async function pick(hit) {
    setPicked(hit); setChosen([]); setGroups([]);
    try {
      const data = await client.get(`/restaurant/products/${hit.product_id}/modifiers`);
      setGroups((data.groups ?? []).filter(group => !group.archived));
    } catch (error) { setMessage(error.message); }
  }

  async function run(action) {
    if (busy || mutation.blocked) return;
    setBusy(true); setMessage(''); setSuccess('');
    try { await action(); } catch (error) {
      setMessage(error.status === 409 ? 'Pesanan berubah di perangkat lain. Tampilan dimuat ulang, coba lagi.' : error.message);
    } finally { setBusy(false); order.reload(); }
  }

  function addLine(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    run(async () => {
      const problem = modifierError(groups, chosen);
      if (problem) throw new Error(problem);
      const body = addLineBody(picked.variant.id, form.get('qty'), form.get('note'), chosen);
      await client.post(`/restaurant/orders/${id}/lines`, body, { version });
      setPicked(null); setQuery(''); setHits([]);
    });
  }

  const removeLine = line => run(async () => {
    await client.del(`/restaurant/orders/${id}/lines/${line.line_id}`, { version });
    setDeleting(null);
  });

  function openAction(setTarget, target) {
    setMessage('');
    setTarget(target);
  }

  function saveLine(event) {
    event.preventDefault();
    const values = Object.fromEntries(new FormData(event.currentTarget));
    run(async () => {
      await client.put(`/restaurant/orders/${id}/lines/${editing.line_id}`, editLineBody(values.qty, values.note), { version });
      setEditing(null);
    });
  }

  function saveDiscount(event) {
    event.preventDefault();
    const values = Object.fromEntries(new FormData(event.currentTarget));
    const suffix = discountTarget.line_id ? `/lines/${discountTarget.line_id}/discount` : '/discount';
    run(async () => {
      await client.put(`/restaurant/orders/${id}${suffix}`, orderDiscountBody(values.kind, values.value, values.reason), { version });
      setDiscountTarget(null);
    });
  }

  function cancelLine(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const line = cancelling;
    run(async () => {
      const body = cancelBody(form.get('reason'), form.get('approval_email'), form.get('approval_pin'));
      await client.post(`/restaurant/orders/${id}/lines/${line.line_id}/cancel`, body, { version });
      setCancelling(null);
      setSuccess(`${line.product_name} dibatalkan.`);
    });
  }

  async function sendOperation(operation) {
    setBusy(true); setMessage('');
    try {
      const result = await mutation.send(operation);
      if (operation.path.endsWith('/checkout')) setPaid(result);
      else setSuccess('Pesanan dikirim ke dapur.');
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); order.reload(); }
  }

  function sendKitchen() {
    if (busy || mutation.blocked) return;
    sendOperation({ path: `/restaurant/orders/${id}/send`, version, key: newIdempotencyKey() });
  }

  function checkout(event) {
    event.preventDefault();
    if (!methodSettings.data || payments.some(p => !configuredMethods.some(m => m.code === p.method && m.enabled))) { setMessage('Pilih metode aktif; pengaturan dimuat ulang.'); methodSettings.reload(); return; }
    if (busy || mutation.blocked) return;
    try {
      if (!register) throw new Error('Pilih terminal dengan shift terbuka untuk menerima pembayaran.');
      sendOperation({ path: `/restaurant/orders/${id}/checkout`, body: { register_id: register.id, payments: splitCheckoutPayments(payments) }, version, key: newIdempotencyKey() });
    } catch (error) { setMessage(error.message); }
  }

  if (paid) {
    return <section className="panel" role="status">
      <h1>Pembayaran berhasil</h1>
      <p>Struk {paid.receipt_number} · total {formatIDR(BigInt(paid.total_cents))}{BigInt(paid.change_cents) > 0n ? ` · kembalian ${formatIDR(BigInt(paid.change_cents))}` : ''}</p>
      <Link className="primary" to="/meja">Kembali ke meja</Link>
    </section>;
  }

  return <>
    <PageHeading title="Pesanan" description="Tambah menu, kirim ke dapur, lalu bayar."><Link to="/meja">← Meja</Link><Link to="/pesanan">Daftar pesanan</Link></PageHeading>
    <RestaurantOrderHeader id={id} resource={header} disabled={busy || mutation.blocked} onChanged={() => { order.reload(); header.reload(); }} />
    <Notice>{message || order.error?.message}</Notice>
    {message.includes('stasiun aktif') && <div className="alert warn">Atur tujuan kategori menu di Pengaturan → Stasiun & kategori dapur, lalu kirim ulang pesanan ini. {['owner', 'manager'].includes(business?.role) ? <Link to="/pengaturan#pengaturan-dapur">Buka pengaturan dapur</Link> : 'Minta Pemilik atau Manajer mengatur stasiun dapur.'}</div>}
    <Notice kind="success">{success}</Notice>
    <Notice>{mutation.error}</Notice>
    {mutation.pending && <div className="alert warn" role="status">Status aksi belum pasti. Jangan membuat aksi baru.<button disabled={busy || mutation.sending} onClick={() => sendOperation(mutation.pending)}>Periksa & coba lagi dengan data yang sama</button></div>}
    <fieldset className="order-workspace" disabled={busy || mutation.blocked || !header.data || header.data.status !== 'open'}>
    <div className="order-main-column">
    <section className="panel order-menu-panel">
      <div className="panel-heading"><div><h2>Tambah menu</h2><p className="muted small">Cari menu untuk ditambahkan ke pesanan ini.</p></div></div>
      <Field label="Cari menu (nama, SKU, barcode)" id="menu-search" value={query} onChange={event => setQuery(event.target.value)} autoComplete="off" placeholder="Ketik nama menu, SKU, atau barcode" />
      {!picked && hits.length > 0 && <ul className="menu-hits">{hits.map(hit => <li key={hit.variant.id}>
        <button onClick={() => pick(hit)}>{hit.product_name}{hit.variant.name ? ` · ${hit.variant.name}` : ''} · {formatIDR(BigInt(hit.variant.price_cents))}</button>
      </li>)}</ul>}
      {picked && <form onSubmit={addLine}><fieldset disabled={busy || version === undefined}>
        <h3>{picked.product_name}</h3>
        <div className="form-grid">
          <Field label="Jumlah" id="qty" name="qty" type="number" min="1" step="1" defaultValue="1" required />
          <Field label="Catatan (maks 140)" id="note" name="note" maxLength={140} />
        </div>
        {groups.map(group => <fieldset key={group.group_id} className="modifier-group">
          <legend>{group.name} (pilih {group.min_choices}–{group.max_choices})</legend>
          {group.options.map(option => <label key={option.option_id} className="check">
            <input type="checkbox" checked={chosen.includes(option.option_id)} onChange={event => setChosen(prev => event.target.checked ? [...prev, option.option_id] : prev.filter(x => x !== option.option_id))} />
            {option.name}{BigInt(option.price_delta_cents) > 0n ? ` (+${formatIDR(BigInt(option.price_delta_cents))})` : ''}
          </label>)}
        </fieldset>)}
        <div className="row"><button className="primary">Tambah ke pesanan</button><button type="button" onClick={() => setPicked(null)}>Batal</button></div>
      </fieldset></form>}
    </section>
    <section className="panel order-items-panel">
      <div className="panel-heading"><div><h2>Item pesanan</h2><p className="muted small">Periksa item dan catatan sebelum dikirim ke dapur.</p></div><span className="cart-count">{liveLines(lines).length}</span></div>
      {!liveLines(lines).length ? <p className="muted">Belum ada item.</p> : <ul className="order-lines">{liveLines(lines).map(line => <li key={line.line_id}>
        <div className="order-line-info">
          <strong>{formatQty(BigInt(line.qty_milli))} × {line.product_name}</strong>
          {line.modifiers?.length > 0 && <small>{line.modifiers.map(m => m.name).join(', ')}</small>}
          {line.note && <small>Catatan: {line.note}</small>}
          <small>{lineStatusLabel(line.kitchen_status)} · {formatIDR(BigInt(line.effective_unit_price_cents))}</small>
        </div>
        <div className="order-line-actions" role="group" aria-label={`Aksi ${line.product_name}`}>
        {line.kitchen_status !== 'NOT_SENT' && line.kitchen_status !== 'SERVED' && <button className="icon-button danger" type="button" disabled={busy || version === undefined} aria-label={`Batalkan ${line.product_name}`} title="Batalkan item" aria-haspopup="dialog" onClick={() => openAction(setCancelling, line)}><Icon name="close" size={18} /></button>}
        <button className="icon-button order-discount-action" type="button" disabled={busy || version === undefined} aria-label={`Diskon ${line.product_name}`} title="Diskon item" aria-haspopup="dialog" onClick={() => openAction(setDiscountTarget, line)}><Icon name="discount" size={18} /></button>
        {line.kitchen_status === 'NOT_SENT' && <><button className="icon-button" type="button" disabled={busy || version === undefined} aria-label={`Edit ${line.product_name}`} title="Edit item" aria-haspopup="dialog" onClick={() => openAction(setEditing, line)}><Icon name="edit" size={18} /></button><button className="icon-button danger" type="button" disabled={busy || version === undefined} aria-label={`Hapus ${line.product_name}`} title="Hapus item" aria-haspopup="dialog" onClick={() => openAction(setDeleting, line)}><Icon name="trash" size={18} /></button></>}
        </div>
      </li>)}</ul>}
      {editing && <OrderActionModal title={`Edit ${editing.product_name}`} busy={busy} error={message} onClose={() => setEditing(null)}><form key={editing.line_id} onSubmit={saveLine}><fieldset disabled={busy || version === undefined}>
        <Field label="Jumlah" id="edit_qty" name="qty" type="number" step="0.001" min="0.001" required defaultValue={formatQty(BigInt(editing.qty_milli)).replace(',', '.')} />
        <Field label="Catatan" id="edit_note" name="note" maxLength={140} defaultValue={editing.note} />
        <div className="row"><button className="primary">Simpan item</button><button type="button" onClick={() => setEditing(null)}>Batal</button></div>
      </fieldset></form></OrderActionModal>}
      {deleting && <OrderActionModal title="Hapus item pesanan" busy={busy} error={message} onClose={() => setDeleting(null)}><form onSubmit={event => { event.preventDefault(); removeLine(deleting); }}><fieldset disabled={busy || version === undefined}>
        <p className="muted">Hapus {formatQty(BigInt(deleting.qty_milli))} × {deleting.product_name} dari pesanan?</p>
        <div className="row"><button type="button" onClick={() => setDeleting(null)}>Batal</button><button className="danger">{busy ? 'Menghapus…' : 'Hapus item'}</button></div>
      </fieldset></form></OrderActionModal>}
      {cancelling && <OrderActionModal title={`Batalkan ${cancelling.product_name}`} busy={busy} error={message} onClose={() => setCancelling(null)}><form onSubmit={cancelLine}><fieldset disabled={busy}>
        <p className="muted small">Item sudah di dapur. {cancelNeedsApproval(cancelling.kitchen_status, business?.role) ? 'Persetujuan atasan (email + PIN) diperlukan.' : 'Alasan tercatat di laporan pembatalan.'}</p>
        <div className="form-grid">
          <Field label="Alasan (wajib, maks 200)" id="reason" name="reason" required maxLength={200} />
          <Field label="Email atasan" id="approval_email" name="approval_email" type="email" required={cancelNeedsApproval(cancelling.kitchen_status, business?.role)} autoComplete="off" />
          <Field label="PIN atasan (6 digit)" id="approval_pin" name="approval_pin" type="password" inputMode="numeric" pattern="[0-9]{6}" maxLength={6} required={cancelNeedsApproval(cancelling.kitchen_status, business?.role)} autoComplete="off" />
        </div>
        <div className="row"><button className="primary">Konfirmasi pembatalan</button><button type="button" onClick={() => setCancelling(null)}>Batal</button></div>
      </fieldset></form></OrderActionModal>}
      <div className="order-kitchen-actions"><button className="primary" disabled={busy || version === undefined || pending === 0} onClick={sendKitchen}>Kirim ke dapur{pending ? ` (${pending})` : ''}</button></div>
    </section>
    </div>
    <section className="panel order-bill-panel">
      <div className="panel-heading"><h2>Tagihan</h2>
      <button disabled={version === undefined} aria-haspopup="dialog" onClick={() => openAction(setDiscountTarget, { product_name: 'pesanan' })}>Diskon pesanan</button>
      </div>
      {discountTarget && <OrderActionModal title={`Atur diskon ${discountTarget.product_name}`} busy={busy} error={message} onClose={() => setDiscountTarget(null)}><form key={discountTarget.line_id ?? 'order'} onSubmit={saveDiscount}><fieldset disabled={busy || version === undefined}>
        <Field label="Jenis diskon" id="discount_kind"><select id="discount_kind" name="kind"><option value="none">Tanpa diskon</option><option value="percent">Persen</option><option value="amount">Nominal Rupiah</option></select></Field>
        <Field label="Nilai diskon (% atau Rp)" id="discount_value" name="value" inputMode="decimal" />
        <Field label="Alasan" id="discount_reason" name="reason" maxLength={200} />
        <div className="row"><button className="primary">Simpan diskon</button><button type="button" onClick={() => setDiscountTarget(null)}>Batal</button></div>
      </fieldset></form></OrderActionModal>}
      {bill.data && <dl className="bill">
        <dt>Subtotal</dt><dd>{formatIDR(BigInt(bill.data.subtotal_gross_cents))}</dd>
        <dt>Diskon item</dt><dd>{formatIDR(BigInt(bill.data.line_discount_cents))}</dd>
        <dt>Diskon pesanan</dt><dd>{formatIDR(BigInt(bill.data.transaction_discount_cents))}</dd>
        <dt>Pajak</dt><dd>{formatIDR(BigInt(bill.data.tax_cents))}</dd>
        <dt>Service charge</dt><dd>{formatIDR(BigInt(bill.data.service_charge_cents))}</dd>
        {BigInt(bill.data.delivery_fee_cents) > 0n && <><dt>Ongkir</dt><dd>{formatIDR(BigInt(bill.data.delivery_fee_cents))}</dd></>}
        <dt className="order-bill-total">Total</dt><dd className="order-bill-total"><strong>{formatIDR(BigInt(bill.data.total_cents))}</strong></dd>
      </dl>}
      {bill.loading && <p className="muted small" role="status">Memuat rincian tagihan…</p>}
      <Notice>{bill.error?.message}</Notice>
      {blocked && <p className="order-payment-notice" role="status">{blocked}</p>}
      <form className="order-payment-form" onSubmit={checkout}><fieldset disabled={busy || version === undefined || !!blocked}>
        {payments.map((payment, index) => <div className="order-payment-entry" key={payment.id}>
          <Field label={`Metode pembayaran ${index + 1}`} id={`method-${payment.id}`}><select id={`method-${payment.id}`} value={payment.method} onChange={event => setPayments(rows => rows.map(row => row.id === payment.id ? { ...row, method: event.target.value } : row))}>{!configuredMethods.some(m => m.code === payment.method && m.enabled) && <option value={payment.method} disabled>Metode nonaktif/belum dimuat — pilih metode aktif</option>}{configuredMethods.filter(m => m.enabled).map(m => <option key={m.code} value={m.code}>{m.name}</option>)}</select></Field>
          <Field label={`Diterima pembayaran ${index + 1} (Rp)`} id={`amount-${payment.id}`} type="number" min="0.01" step="0.01" required value={payment.amount} onChange={event => setPayments(rows => rows.map(row => row.id === payment.id ? { ...row, amount: event.target.value } : row))} />
          {payments.length > 1 && <button type="button" onClick={() => setPayments(rows => rows.filter(row => row.id !== payment.id))}>Hapus pembayaran {index + 1}</button>}
        </div>)}
        <p className="muted small">Kembalian hanya dari tunai. Nominal dan batas diskon diperiksa kembali di server.</p>
        <div className="order-payment-actions"><button type="button" onClick={() => setPayments(rows => [...rows, { id: newIdempotencyKey(), method: 'cash', amount: '' }])}>Tambah metode pembayaran</button><button className="primary">Bayar</button></div>
      </fieldset></form>
    </section>
    </fieldset>
  </>;
}
