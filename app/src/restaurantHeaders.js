import { rupiahToCents } from './forms.js';

export function restaurantHeaderBody(type, values) {
  if (!['DINE_IN', 'TAKEAWAY', 'DELIVERY_MANUAL'].includes(type)) throw new Error('Jenis pesanan tidak valid.');
  const text = key => String(values[key] ?? '').trim();
  const body = { note: text('note'), channel: '', delivery_fee_cents: '0', contact_name: '', contact_phone: '', queue_label: '' };
  if ([...body.note].length > 200) throw new Error('Catatan maksimal 200 karakter.');
  if (type !== 'DINE_IN') {
    body.contact_name = text('contact_name'); body.queue_label = text('queue_label');
    if ([...body.contact_name].length > 100 || [...body.queue_label].length > 40) throw new Error('Nama maksimal 100 dan antrean maksimal 40 karakter.');
  }
  if (type === 'DELIVERY_MANUAL') {
    body.channel = text('channel'); body.contact_phone = text('contact_phone');
    body.delivery_fee_cents = rupiahToCents(values.fee || '0');
    if ([...body.channel].length > 100 || body.contact_phone.length > 32 || !/^[0-9+ ()-]*$/.test(body.contact_phone)) throw new Error('Kanal/telepon tidak valid.');
    if (BigInt(body.delivery_fee_cents) > 10000000000n) throw new Error('Ongkir maksimal Rp100 juta.');
  }
  return body;
}
