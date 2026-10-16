import { useState } from "react";
import { Link } from 'react-router-dom';
import { useSession } from "../auth/SessionContext.jsx";
import { useWorkspace } from "../workspace/WorkspaceContext.jsx";
import { useResource } from "../hooks/useResource.js";
import { formatIDR, formatQty } from "../money.js";
import { PageHeading, ResourceState, Notice } from "../components/UI.jsx";
import { Icon } from "../components/Icon.jsx";
import { localDate } from "../reporting.js";
import StockOperations from "./StockOperations.jsx";
export const reportTypes = {
  sales: {
    title: "Penjualan",
    columns: [["date", "Tanggal"], ["sale_count", "Transaksi"], ["gross_cents", "Bruto"], ["discount_cents", "Diskon"], ["returns_cents", "Retur"], ["net_cents", "Neto"], ["tax_cents", "Pajak"], ["service_charge_cents", "Service charge"], ["delivery_fee_cents", "Ongkir neto"], ["cash_rounding_cents", "Pembulatan"], ["net_bill_cents", "Tagihan neto setelah retur"]]
  },
  payments: {
    title: "Pembayaran",
    columns: [["date", "Tanggal"], ["method", "Metode"], ["verification", "Verifikasi"], ["received_cents", "Penerimaan"], ["refunded_cents", "Refund"], ["net_cents", "Neto"]]
  },
  products: {
    title: "Performa produk",
    columns: [["product_name", "Produk"], ["sold_base_qty_milli", "Terjual"], ["returned_base_qty_milli", "Diretur"], ["net_sales_cents", "Penjualan neto"], ["estimated_gross_profit_cents", "Estimasi laba kotor"]]
  },
  corrections: {
    title: "Koreksi & diskon",
    columns: [["local_date", "Tanggal"], ["kind", "Jenis"], ["receipt_number", "Nomor struk"], ["reason", "Alasan"], ["amount_cents", "Nominal"]]
  },
  stock: {
    title: "Persediaan",
    columns: [["product_name", "Produk"], ["sku", "SKU"], ["base_unit", "Satuan"], ["qty_milli", "Stok"], ["minimum_qty_milli", "Minimum"], ["estimated_value_cents", "Estimasi nilai"]]
  }
};
function cell(value, key) {
  if (value == null) return "—";
  if (key.endsWith("_cents")) return formatIDR(BigInt(value));
  if (key.endsWith("_milli")) return formatQty(BigInt(value));
  return String(value);
}
export default function ReportsPage({
  stockOnly = false
}) {
  const {
    client
  } = useSession();
  const {
    branch
  } = useWorkspace();
  const [type, setType] = useState(stockOnly ? "stock" : "sales");
  const [from, setFrom] = useState(localDate(branch?.timezone));
  const [to, setTo] = useState(localDate(branch?.timezone));
  const [filter, setFilter] = useState("all");
  const [group, setGroup] = useState('product');
  const [exportJob, setExportJob] = useState(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const query = new URLSearchParams({
    branch_id: branch?.id ?? "",
    from,
    to,
    ...(type === 'products' ? { group_by: group } : {}),
    ...(type === "stock" ? {
      filter
    } : {})
  });
  const path = branch && from && to && from <= to ? `/reports/${type}?${query}` : null;
  const resource = useResource(path);
  const categories = useResource(type === 'products' && group === 'category' ? '/categories' : null);
  const categoryNames = new Map((categories.data?.categories ?? []).map(item => [item.id, item.name]));
  const rows = resource.data?.rows ?? [];
  const config = reportTypes[type];
  const baseColumns = type === 'products' && group === 'category'
    ? [['category_id', 'Kategori'], ['product_count', 'Jumlah produk'], ['net_sales_cents', 'Penjualan neto'], ['estimated_gross_profit_cents', 'Estimasi laba kotor']]
    : config.columns;
  const columns = baseColumns.filter(([key]) => !(key === "estimated_gross_profit_cents" && !resource.data?.profit_visible) && !(key === "estimated_value_cents" && !resource.data?.cost_visible));
  function saveBlob(blob, filename) {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  async function download() {
    setBusy(true);
    setMessage("");
    try {
      const blob = await client.get(`${path}&format=csv`, {
        responseType: "blob"
      });
      saveBlob(blob, `${type}-${from}-${to}.csv`);
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }
  async function backgroundExport() {
    setBusy(true);
    setMessage('');
    try {
      const result = await client.post(`/reports/${type}/exports?${query}`, {});
      setExportJob({ ...result, filename: `${type}-${from}-${to}.csv` });
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }
  async function checkExport() {
    setBusy(true);
    setMessage('');
    try {
      const result = await client.get(`/report-exports/${exportJob.export_id}`);
      setExportJob(current => ({ ...current, ...result }));
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }
  async function downloadExport() {
    setBusy(true);
    setMessage('');
    try {
      saveBlob(await client.get(`/report-exports/${exportJob.export_id}?download=true`, { responseType: 'blob' }), exportJob.filename);
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }
  return <>
  <PageHeading title={stockOnly ? "Persediaan" : "Laporan usaha"} description={stockOnly ? "Pantau saldo stok terkini dan kebutuhan pengadaan." : "Angka penjualan dan pembayaran sesuai periode cabang."}><div className="row"><button onClick={backgroundExport} disabled={!path || busy}>Ekspor latar belakang</button><button className="primary" onClick={download} disabled={!path || !!resource.error || resource.loading || busy}>
      {busy ? "Menyiapkan…" : "Ekspor CSV"}
      <Icon name="arrow" size={16} />
    </button></div></PageHeading>
  <p><Link to="/ekspor">Buka riwayat dan unduh ekspor sebelumnya</Link></p>
  <Notice>{message}</Notice>
  {exportJob && <section className="panel form-panel"><div className="table-toolbar"><div><h2>Ekspor laporan</h2><p className="muted">{({ pending: 'Menunggu antrean', processing: 'Sedang diproses', completed: 'Siap diunduh', failed: 'Ekspor gagal', expired: 'Hasil kedaluwarsa' })[exportJob.status] ?? exportJob.status}{exportJob.row_count != null ? ` · ${exportJob.row_count} baris` : ''}</p>{exportJob.error_code && <p role="alert">{exportJob.error_code}</p>}</div><div className="row"><button onClick={checkExport} disabled={busy}>Periksa status</button>{exportJob.status === 'completed' && <button className="primary" onClick={downloadExport} disabled={busy}>Unduh hasil CSV</button>}</div></div></section>}
  {stockOnly && <StockOperations key={branch?.id ?? 'no-branch'} onChanged={resource.reload} />}
  <section className="panel">
    <div className="table-toolbar">
      <div className="filter-fields">
        {!stockOnly && <label>Jenis laporan<select value={type} onChange={e => setType(e.target.value)}>{Object.entries(reportTypes).map(([key, value]) => <option key={key} value={key}>{value.title}</option>)}</select></label>}
        {type === 'products' && <label>Kelompokkan<select value={group} onChange={event => setGroup(event.target.value)}><option value="product">Per produk</option><option value="category">Per kategori</option></select></label>}
        <label>Dari<input type="date" value={from} max={to} onChange={e => setFrom(e.target.value)} /></label>
        <label>Sampai<input type="date" value={to} min={from} onChange={e => setTo(e.target.value)} /></label>
        {type === "stock" && <label>Stok<select value={filter} onChange={e => setFilter(e.target.value)}>
            <option value="all">Semua stok</option>
            <option value="below_minimum">Perlu dipesan</option>
            <option value="negative">Stok negatif</option>
          </select></label>}
      </div>
      <button className="icon-button" aria-label="Muat ulang laporan" onClick={resource.reload}><Icon name="refresh" /></button>
    </div>
    {!branch ? <div className="empty-state">Pilih cabang untuk melihat laporan.</div> : from > to ? <Notice>Tanggal awal harus sebelum tanggal akhir.</Notice> : <ResourceState resource={resource} empty={!rows.length} message="Tidak ada aktivitas pada periode ini. Pilih periode lain atau mulai mencatat transaksi.">
      {resource.data?.valuation_label && <p className="report-note">{resource.data.valuation_label}</p>}
      {type === "stock" && <p className="report-note">Saldo dan nilai stok adalah kondisi terkini. Periode hanya memfilter pergerakan.</p>}
      <div className="table-scroll"><table>
          <thead><tr>{columns.map(([key, title]) => <th key={key} className={key.endsWith("_cents") || key.endsWith("_milli") ? "num" : ""}>{title}</th>)}</tr></thead>
          <tbody>{rows.map((row, index) => <tr key={index}>{columns.map(([key]) => <td key={key} className={key.endsWith("_cents") || key.endsWith("_milli") ? "num" : ""}>{key === 'category_id' ? (row[key] == null ? 'Tanpa kategori' : categoryNames.get(row[key]) ?? row[key]) : cell(row[key], key)}</td>)}</tr>)}</tbody>
        </table></div>
      <div className="table-footer">{rows.length} baris · {branch.timezone}</div>
    </ResourceState>}
  </section>
</>;
}
