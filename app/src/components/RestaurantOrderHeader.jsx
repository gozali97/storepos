import { useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';
import { restaurantHeaderBody } from '../restaurantHeaders.js';
import { discountToInput } from '../cashier.js';
import { formatIDR } from '../money.js';
import { Field, Notice, ResourceState } from './UI.jsx';

export function RestaurantOrderHeader({ id, resource, disabled, onChanged }) {
  const { client, business } = useSession();
  const [mode, setMode] = useState(''); const [busy, setBusy] = useState(false); const [message, setMessage] = useState('');
  const data = resource.data;
  async function submit(event) {
    event.preventDefault(); if (busy || disabled || !data) return;
    const v = Object.fromEntries(new FormData(event.currentTarget));
    setBusy(true); setMessage('');
    try {
      if (mode === 'edit') await client.put(`/restaurant/orders/${id}`, restaurantHeaderBody(data.order_type, v), { version: data.version });
      if (mode === 'cancel') await client.post(`/restaurant/orders/${id}/cancel`, { reason: v.reason.trim() }, { version: data.version });
      if (mode === 'erase') await client.del(`/restaurant/orders/${id}/contact`, { version: data.version });
      setMode(''); onChanged();
    } catch (error) { setMessage(error.message); if (error.status === 409) { setMode(''); onChanged(); } }
    finally { setBusy(false); }
  }
  return <section className="panel"><ResourceState resource={resource}>{data && <>
    <h2>{data.order_type} · {data.status}/{data.payment_status}</h2><p>Antrean: {data.queue_label || '—'} · Kanal: {data.channel || '—'} · Ongkir {formatIDR(BigInt(data.delivery_fee_cents))}</p><p>{data.note}</p>
    <div className="row"><button disabled={busy || disabled} onClick={() => { setMode(''); resource.reload(); }}>Muat ulang header</button>{data.status === 'open' && <><button disabled={busy || disabled} onClick={() => setMode('edit')}>Edit header/kontak</button><button disabled={busy || disabled} onClick={() => setMode('cancel')}>Batalkan pesanan</button></>}{business?.role === 'owner' && <button disabled={busy || disabled} onClick={() => setMode('erase')}>Hapus kontak/antrean</button>}</div>
    <Notice>{message}</Notice>
    {mode && <form key={`${mode}-${data.version}`} onSubmit={submit}><fieldset disabled={busy || disabled}>
      {mode === 'edit' && <><Field label="Catatan" id="header_note" name="note" maxLength={200} defaultValue={data.note} />
        {data.order_type !== 'DINE_IN' && <><Field label="Nama penerima/pelanggan" id="contact_name" name="contact_name" maxLength={100} defaultValue={data.contact_name} /><Field label="Nomor antrean" id="queue_label" name="queue_label" maxLength={40} defaultValue={data.queue_label} /></>}
        {data.order_type === 'DELIVERY_MANUAL' && <><Field label="Kanal" id="channel" name="channel" maxLength={100} defaultValue={data.channel} /><Field label="Telepon penerima" id="contact_phone" name="contact_phone" maxLength={32} defaultValue={data.contact_phone} /><Field label="Ongkir (Rp)" id="fee" name="fee" type="number" min="0" max="100000000" step="0.01" defaultValue={discountToInput({ kind: 'amount', value: data.delivery_fee_cents })} /></>}
      </>}
      {mode === 'cancel' && <><p>Batalkan item dapur yang sudah dikirim dengan otorisasi terlebih dahulu. Server menolak bila item dapur masih aktif.</p><Field label="Alasan pembatalan" id="order_cancel_reason" name="reason" required maxLength={200} /></>}
      {mode === 'erase' && <><p>Nama, telepon, dan antrean akan dikosongkan tanpa menghapus pesanan.</p><label><input type="checkbox" required /> Saya mengonfirmasi penghapusan kontak pesanan ini</label></>}
      <div className="row"><button className="primary">Konfirmasi {mode === 'edit' ? 'simpan' : mode === 'cancel' ? 'pembatalan' : 'hapus kontak'}</button><button type="button" onClick={() => setMode('')}>Batal</button></div>
    </fieldset></form>}
  </>}</ResourceState></section>;
}
