import { useRef, useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';
import { Field, Notice } from '../components/UI.jsx';
import { productVariants } from '../variants.js';

export default function VariantBuilder({ categories, onCreated, onClose }) {
  const { client } = useSession();
  const form = useRef(null);
  const [attributes, setAttributes] = useState([{ name: '', values: '' }]);
  const [variants, setVariants] = useState([]);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');

  function updateAttribute(index, patch) {
    setAttributes(current => current.map((item, i) => i === index ? { ...item, ...patch } : item));
    setVariants([]);
  }
  async function generate() {
    if (!form.current.reportValidity()) return;
    const values = new FormData(form.current);
    setBusy(true); setMessage('');
    try {
      const result = await client.post('/catalog/variant-combinations', {
        sku_prefix: String(values.get('sku_prefix')).trim(),
        attributes: attributes.map(attribute => ({ name: attribute.name.trim(), values: attribute.values.split(',').map(value => value.trim()) })),
      });
      setVariants(result.variants.map(variant => ({ ...variant, price: values.get('default_price') })));
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }
  function updateVariant(index, patch) { setVariants(current => current.map((variant, i) => i === index ? { ...variant, ...patch } : variant)); }
  async function save(event) {
    event.preventDefault();
    if (!variants.length) return;
    const values = new FormData(event.currentTarget);
    setBusy(true); setMessage('');
    try {
      await client.post('/products', {
        name: String(values.get('product_name')).trim(), category_id: values.get('category_id') || null,
        track_stock: values.get('track_stock') === 'on',
        variants: productVariants(variants, Object.fromEntries(values)),
      });
      onCreated();
    } catch (error) { setMessage(error.message); }
    finally { setBusy(false); }
  }

  return <section className="panel form-panel"><div className="panel-heading"><div><h2>Produk dengan banyak varian</h2><p className="muted small">Maksimal 3 atribut dan 100 kombinasi. Periksa SKU dan harga sebelum menyimpan.</p></div><button disabled={busy} onClick={onClose}>Tutup</button></div><Notice>{message}</Notice>
    <form ref={form} onSubmit={save}><fieldset disabled={busy}><div className="form-grid">
      <Field label="Nama produk" id="variant_product_name"><input id="variant_product_name" name="product_name" required maxLength={200} /></Field>
      <Field label="Awalan SKU" id="variant_prefix"><input id="variant_prefix" name="sku_prefix" required maxLength={100} placeholder="BAJU" /></Field>
      <Field label="Satuan dasar" id="variant_base_unit"><input id="variant_base_unit" name="base_unit" required placeholder="pcs" /></Field>
      <Field label="Harga awal per varian (Rp)" id="variant_default_price"><input id="variant_default_price" name="default_price" type="number" min="0" step="0.01" required /></Field>
      <Field label="Kategori" id="variant_category"><select id="variant_category" name="category_id"><option value="">Tanpa kategori</option>{categories.map(category => <option key={category.id} value={category.id}>{category.name}</option>)}</select></Field>
    </div><div className="checkbox-row"><label><input name="track_stock" type="checkbox" defaultChecked />Lacak stok</label><label><input name="fractional" type="checkbox" />Izinkan kuantitas pecahan</label><label><input name="includes_tax" type="checkbox" />Harga termasuk pajak</label></div>
      <div className="variant-attributes">{attributes.map((attribute, index) => <div key={index}><label>Nama atribut {index + 1}<input value={attribute.name} onChange={event => updateAttribute(index, { name: event.target.value })} required maxLength={100} placeholder="Warna" /></label><label>Nilai, pisahkan dengan koma<input value={attribute.values} onChange={event => updateAttribute(index, { values: event.target.value })} required placeholder="Biru, Putih, Hitam" /></label><button type="button" disabled={attributes.length === 1} onClick={() => { setAttributes(current => current.filter((_, i) => i !== index)); setVariants([]); }}>Hapus atribut</button></div>)}</div>
      <div className="row"><button type="button" disabled={attributes.length >= 3} onClick={() => { setAttributes(current => [...current, { name: '', values: '' }]); setVariants([]); }}>Tambah atribut</button><button type="button" onClick={generate}>Buat pratinjau kombinasi</button></div>
      {variants.length > 0 && <><p className="report-note">{variants.length} kombinasi siap diperiksa. Perubahan atribut akan menghapus pratinjau.</p><div className="table-scroll"><table><thead><tr><th>Kombinasi</th><th>SKU</th><th>Harga jual (Rp)</th></tr></thead><tbody>{variants.map((variant, index) => <tr key={index}><td>{variant.name}</td><td><input aria-label={`SKU ${variant.name}`} value={variant.sku} maxLength={100} required onChange={event => updateVariant(index, { sku: event.target.value })} /></td><td><input aria-label={`Harga ${variant.name}`} type="number" min="0" step="0.01" required value={variant.price} onChange={event => updateVariant(index, { price: event.target.value })} /></td></tr>)}</tbody></table></div><button className="primary">Simpan produk & semua varian</button></>}
    </fieldset></form>
  </section>;
}
