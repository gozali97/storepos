import { useState } from "react";
import { Link } from 'react-router-dom';
import { useSession } from "../auth/SessionContext.jsx";
import { useResource } from "../hooks/useResource.js";
import { termsFor } from "../terms.js";
import { formatIDR } from "../money.js";
import { rupiahToCents } from "../forms.js";
import { PageHeading, ResourceState, Field, Notice } from "../components/UI.jsx";
import { Icon } from "../components/Icon.jsx";
import ProductUnits from './ProductUnits.jsx';
import VariantBuilder from './VariantBuilder.jsx';
import BarcodeLabels from './BarcodeLabels.jsx';
export default function ProductsPage() {
  const {
    client,
    business
  } = useSession();
  const terms = termsFor(business?.business_type);
  const canManage = ["owner", "manager", "pharmacist"].includes(business?.role);
  const [unitProduct, setUnitProduct] = useState(null);
  const [query, setQuery] = useState("");
  const [search, setSearch] = useState("");
  const [show, setShow] = useState(false);
  const [showVariants, setShowVariants] = useState(false);
  const [showLabels, setShowLabels] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [success, setSuccess] = useState("");
  const resource = useResource(`/catalog/search?q=${encodeURIComponent(search)}&limit=100`);
  const categories = useResource("/categories");
  const items = resource.data?.items ?? [];
  async function create(event) {
    event.preventDefault();
    setBusy(true);
    setMessage("");
    setSuccess("");
    const values = Object.fromEntries(new FormData(event.currentTarget));
    try {
      await client.post("/products", {
        name: values.name.trim(),
        category_id: values.category || null,
        track_stock: values.track_stock === "on",
        variants: [{
          name: "",
          sku: values.sku.trim(),
          base_unit: values.base_unit.trim(),
          price_cents: rupiahToCents(values.price),
          barcodes: values.barcode.trim() ? [values.barcode.trim()] : [],
          allow_fractional: values.fractional === "on",
          price_includes_tax: values.includes_tax === "on"
        }]
      });
      setShow(false);
      setSuccess(`${terms.product} berhasil ditambahkan.`);
      resource.reload();
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }
  async function updatePrice(event, item) {
    event.preventDefault();
    setMessage("");
    setSuccess("");
    setBusy(true);
    try {
      await client.patch(`/variants/${item.variant.id}/price`, {
        price_cents: rupiahToCents(new FormData(event.currentTarget).get("price"))
      }, {
        version: item.variant.version
      });
      resource.reload();
      setSuccess("Harga berhasil diperbarui.");
    } catch (error) {
      setMessage(error.status === 409 ? "Harga diubah pengguna lain. Muat ulang katalog sebelum mencoba lagi." : error.message);
    } finally {
      setBusy(false);
    }
  }
  return <>
  <PageHeading title={terms.products} description="Kelola katalog, harga jual, SKU, dan barcode usaha Anda.">{canManage && <div className="row"><button onClick={() => { setShowLabels(value => !value); setShow(false); setShowVariants(false); }}>{showLabels ? 'Tutup label' : 'Cetak label barcode'}</button><button onClick={() => { setShowVariants(value => !value); setShow(false); setShowLabels(false); }}>Produk banyak varian</button><button className="primary" onClick={() => { setShow(v => !v); setShowVariants(false); setShowLabels(false); }}>
      <Icon name={show ? "close" : "plus"} size={17} />
      {show ? "Tutup formulir" : `Tambah ${terms.product.toLowerCase()}`}
    </button></div>}</PageHeading>
  {canManage && business?.business_type === 'retail' && <p><Link to="/impor-katalog">Impor produk & stok awal lewat CSV</Link></p>}
  <Notice>{message}</Notice>
  <Notice kind="success">{success}</Notice>
  {showVariants && canManage && <VariantBuilder categories={categories.data?.categories ?? []} onClose={() => setShowVariants(false)} onCreated={() => { setShowVariants(false); setSuccess('Produk dan seluruh varian disimpan.'); resource.reload(); }} />}
  {showLabels && canManage && <BarcodeLabels items={items} onClose={() => setShowLabels(false)} />}
  {unitProduct && canManage && <ProductUnits key={unitProduct.variant.id} product={unitProduct} onClose={() => setUnitProduct(null)} />}
  {show && <section className="panel form-panel">
    <h2>{`Tambah ${terms.product.toLowerCase()}`}</h2>
    <form onSubmit={create}>
      <div className="form-grid">
        <Field label="Nama" id="name" required maxLength={200} />
        <Field label="SKU" id="sku" required maxLength={100} />
        <Field label="Satuan dasar" id="base_unit" required placeholder="pcs, kg, tablet" />
        <Field label="Harga jual (Rp)" id="price" type="number" min="0" step="0.01" required />
        <Field label="Barcode (opsional)" id="barcode" />
        <Field label="Kategori" id="category"><select id="category" name="category">
            <option value="">Tanpa kategori</option>
            {categories.data?.categories?.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select></Field>
      </div>
      <div className="checkbox-row">
        <label><input type="checkbox" name="track_stock" defaultChecked />Lacak stok</label>
        <label><input type="checkbox" name="fractional" />Izinkan kuantitas pecahan</label>
        <label><input type="checkbox" name="includes_tax" />Harga termasuk pajak</label>
      </div>
      <button className="primary" disabled={busy}>{busy ? "Menyimpan…" : "Simpan produk"}</button>
    </form>
  </section>}
  <section className="panel">
    <div className="table-toolbar">
      <div>
        <h2>Katalog usaha</h2>
        <p className="muted small">{items.length} hasil ditampilkan (maksimal 100)</p>
      </div>
      <form className="search-form" onSubmit={e => {
          e.preventDefault();
          setSearch(query);
        }}>
        <Icon name="search" size={18} />
        <input aria-label="Cari produk, SKU atau barcode" placeholder="Cari nama, SKU, barcode…" value={query} onChange={e => setQuery(e.target.value)} />
        <button type="submit">Cari</button>
      </form>
    </div>
    <ResourceState resource={resource} empty={!items.length} message={search ? "Tidak ada produk yang cocok. Coba kata pencarian lain." : "Tambahkan produk pertama untuk mulai membuat katalog."}><div className="table-scroll"><table>
          <thead><tr>
              <th>Nama & varian</th>
              <th>SKU</th>
              <th>Satuan</th>
              <th>Barcode</th>
              <th className="num">Harga jual</th>
              {canManage && <th>Ubah harga (Rp)</th>}
              {canManage && <th>Satuan tambahan</th>}
            </tr></thead>
          <tbody>{items.map(item => <tr key={item.variant.id}>
              <td><span className="product-name">
                  <span className="product-avatar"><Icon name="box" /></span>
                  <span>
                    <strong>{item.product_name}</strong>
                    <small>{item.variant.name || "Varian utama"}</small>
                  </span>
                </span></td>
              <td>{item.variant.sku}</td>
              <td>{item.variant.base_unit}</td>
              <td>{item.variant.barcodes?.join(", ") || "—"}</td>
              <td className="num">{formatIDR(BigInt(item.variant.price_cents))}</td>
              {canManage && <td><form className="price-form" onSubmit={e => updatePrice(e, item)} key={`${item.variant.id}-${item.variant.version}`}>
                  <input aria-label={`Harga ${item.product_name}`} name="price" type="number" min="0" step="0.01" required defaultValue={(BigInt(item.variant.price_cents) / 100n).toString() + "." + (BigInt(item.variant.price_cents) % 100n).toString().padStart(2, "0")} />
                  <button disabled={busy}>Simpan</button>
                </form></td>}
              {canManage && <td><button onClick={() => setUnitProduct(item)}>Kelola satuan</button></td>}
            </tr>)}</tbody>
        </table></div></ResourceState>
  </section>
</>;
}
