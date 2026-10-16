import { useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';
import { useResource } from '../hooks/useResource.js';
import { quantityToMilli, discountToInput } from '../cashier.js';
import { rupiahToCents } from '../forms.js';
import { formatIDR, formatQty } from '../money.js';
import { Field, Notice, ResourceState } from '../components/UI.jsx';

export default function ProductUnits({ product, onClose }) {
  const { client } = useSession();
  const resource = useResource(`/variants/${product.variant.id}/units`);
  const defaults = useResource(`/variants/${product.variant.id}/unit-defaults`);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [success, setSuccess] = useState('');

  async function save(event, action, successMessage) {
    event.preventDefault();
    const form = event.currentTarget;
    const values = Object.fromEntries(new FormData(form));
    setBusy(true); setMessage(''); setSuccess('');
    try {
      await action(values);
      resource.reload(); defaults.reload(); form.reset(); setSuccess(successMessage);
    } catch (error) { if (error.status === 409) defaults.reload(); setMessage(error.message); }
    finally { setBusy(false); }
  }

  return <section className="panel form-panel">
    <div className="panel-heading"><div><h2>Satuan {product.product_name}</h2><p className="muted small">{product.variant.sku} · satuan dasar {product.variant.base_unit}</p></div><button onClick={onClose} disabled={busy}>Tutup</button></div>
    <Notice>{message}</Notice><Notice kind="success">{success}</Notice>
    <form onSubmit={event => save(event, values => client.post('/purchase-units', {
      variant_id: product.variant.id, name: values.unit_name.trim(), factor_milli: quantityToMilli(values.factor),
    }), 'Satuan tambahan dibuat.')}><fieldset disabled={busy}><div className="form-grid">
      <Field label="Nama satuan tambahan" id="unit_name" required placeholder="Dus, pak, botol" />
      <Field label={`Isi dalam ${product.variant.base_unit}`} id="factor" type="number" min="0.001" step="0.001" required placeholder="12" />
    </div><button className="primary">Tambah satuan</button></fieldset></form>
    <ResourceState resource={defaults}>{defaults.data && <form key={defaults.data.version} onSubmit={event => save(event, values => client.put(`/variants/${product.variant.id}/unit-defaults`, { default_purchase_unit_id: values.purchase || null, default_sale_unit_id: values.sale || null }, { version: defaults.data.version }), 'Satuan default disimpan.')}><fieldset disabled={busy || resource.loading || !!resource.error}>
      <h3>Satuan default</h3>
      <Field label="Default pembelian" id="default_purchase"><select name="purchase" id="default_purchase" defaultValue={defaults.data.default_purchase_unit_id ?? ''}><option value="">Satuan dasar</option>{resource.data?.units?.filter(unit => !unit.archived).map(unit => <option key={unit.unit_id} value={unit.unit_id}>{unit.name}</option>)}</select></Field>
      <Field label="Default penjualan" id="default_sale"><select name="sale" id="default_sale" defaultValue={defaults.data.default_sale_unit_id ?? ''}><option value="">Satuan dasar</option>{resource.data?.units?.filter(unit => !unit.archived && unit.sale_price_cents != null).map(unit => <option key={unit.unit_id} value={unit.unit_id}>{unit.name}</option>)}</select></Field>
      <button className="primary">Simpan default</button>
    </fieldset></form>}</ResourceState>
    <p className="report-note">Faktor dikunci setelah satuan dipakai. Isi harga jual agar satuan dapat digunakan pada transaksi penjualan.</p>
    <ResourceState resource={resource} empty={!resource.data?.units?.length} message="Belum ada satuan tambahan untuk produk ini."><div className="table-scroll"><table><thead><tr><th>Satuan</th><th>Konversi</th><th className="num">Harga jual</th><th>Ubah harga (Rp)</th><th>Tambah barcode</th></tr></thead><tbody>{resource.data?.units?.map(unit => <tr key={unit.unit_id}><td><strong>{unit.name}</strong>{unit.factor_locked && <small className="table-subtext">Faktor terkunci</small>}</td><td>{formatQty(BigInt(unit.factor_milli))} {product.variant.base_unit}</td><td className="num">{unit.sale_price_cents == null ? 'Belum diatur' : formatIDR(BigInt(unit.sale_price_cents))}</td><td><form className="price-form" key={`${unit.unit_id}-${unit.sale_price_cents}`} onSubmit={event => save(event, values => client.put(`/purchase-units/${unit.unit_id}/sale-price`, { sale_price_cents: rupiahToCents(values.price) }), 'Harga jual satuan disimpan.')}><input name="price" type="number" min="0" step="0.01" aria-label={`Harga satuan ${unit.name}`} defaultValue={unit.sale_price_cents == null ? '' : discountToInput({ kind: 'amount', value: unit.sale_price_cents })} required disabled={busy} /><button disabled={busy}>Simpan</button></form></td><td><form className="price-form" onSubmit={event => save(event, values => client.post(`/purchase-units/${unit.unit_id}/barcodes`, { barcode: values.barcode.trim() }), 'Barcode satuan ditambahkan.')}><input name="barcode" maxLength={200} aria-label={`Barcode satuan ${unit.name}`} autoComplete="off" required disabled={busy} /><button disabled={busy}>Tambah</button></form></td></tr>)}</tbody></table></div></ResourceState>
  </section>;
}
