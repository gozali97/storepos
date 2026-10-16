import { Link } from "react-router-dom";
import { OnboardingChecklist } from "../components/OnboardingChecklist.jsx";
import { useSession } from "../auth/SessionContext.jsx";
import { useWorkspace } from "../workspace/WorkspaceContext.jsx";
import { useResource } from "../hooks/useResource.js";
import { Icon } from "../components/Icon.jsx";
import { ResourceState } from "../components/UI.jsx";
import { formatIDR, formatQty } from "../money.js";
import { localDate, summarizeSales } from "../reporting.js";
export default function HomePage() {
  const {
    user,
    business
  } = useSession();
  const {
    branch,
    register
  } = useWorkspace();
  const canReport = ["owner", "manager", "pharmacist"].includes(business?.role);
  const today = localDate(branch?.timezone);
  const start = new Date(`${today}T12:00:00Z`);
  start.setUTCDate(start.getUTCDate() - 6);
  const from = start.toISOString().slice(0, 10);
  const query = new URLSearchParams({
    branch_id: branch?.id ?? "",
    from,
    to: today
  });
  const enabled = branch && canReport;
  const sales = useResource(enabled ? `/reports/sales?${query}` : null);
  const products = useResource(enabled ? `/reports/products?${query}` : null);
  const payments = useResource(enabled ? `/reports/payments?${query}` : null);
  const rows = sales.data?.rows ?? [];
  const stats = sales.data ? summarizeSales(rows) : null;
  const days = Array.from({
    length: 7
  }, (_, i) => {
    const date = new Date(start);
    date.setUTCDate(date.getUTCDate() + i);
    const day = date.toISOString().slice(0, 10);
    const sum = summarizeSales(rows.filter(row => row.date === day)).net;
    return {
      day,
      total: sum,
      label: new Intl.DateTimeFormat("id-ID", {
        weekday: "short",
        timeZone: "UTC"
      }).format(date)
    };
  });
  const max = days.reduce((a, d) => d.total != null && d.total > a ? d.total : a, 0n);
  const name = user?.name?.split(" ")[0] ?? user?.email?.split("@")[0] ?? "Anda";
  const selling = ["owner", "manager", "cashier", "pharmacist"].includes(business?.role);
  const restaurant = business?.business_type === "restaurant";
  const quickLinks = [
    ...(selling ? [["/kasir", "Buka kasir", "Mulai transaksi pelanggan", "cart"]] : []),
    ...(restaurant && ["owner", "manager", "cashier"].includes(business?.role) ? [["/meja", "Pantau meja", "Lihat meja dan pesanan aktif", "store"]] : []),
    ...(restaurant && ["owner", "manager", "cashier", "kitchen"].includes(business?.role) ? [["/dapur", "Layar dapur", "Proses antrean masakan", "clock"]] : []),
    ...(business?.business_type === "pharmacy" && ["owner", "manager", "warehouse", "pharmacist"].includes(business?.role) ? [["/batch", "Batch & kedaluwarsa", "Kelola persediaan obat", "box"]] : []),
    ["/produk", restaurant ? "Katalog menu" : "Katalog produk", "Lihat produk dan harga", "box"],
    ...(canReport ? [["/laporan", "Laporan usaha", "Telusuri performa lebih detail", "chart"]] : []),
  ];
  const cards = [["Penjualan neto", stats?.net, "wallet", "Setelah diskon dan retur"], ["Transaksi", stats?.count, "receipt", "Transaksi selesai"], ["Diskon", stats?.discount, "discount", "Diskon baris & transaksi"], ["Retur", stats?.returns, "refresh", "Dicatat pada tanggal retur"]];
  const period = `${new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", timeZone: "UTC" }).format(start)} – ${new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", year: "numeric", timeZone: "UTC" }).format(new Date(`${today}T12:00:00Z`))}`;
  return <div className="dashboard-page">
  <div className="dashboard-intro">
    <div>
      <h1>Dashboard</h1>
      <p className="muted">Performa usaha dan operasional cabang Anda.</p>
    </div>
    <span className="date-label">
      <Icon name="clock" size={17} />
      {new Intl.DateTimeFormat("id-ID", {
          dateStyle: "long",
          timeZone: branch?.timezone ?? "Asia/Jakarta"
        }).format(new Date())}
    </span>
  </div>
  <section className="dashboard-welcome" aria-label="Ruang kerja aktif">
    <div className="dashboard-greeting">
      <span className="dashboard-business"><Icon name="store" size={16} />{business?.business_name ?? "Ruang kerja StorePOS"}</span>
      <h2>Selamat bekerja, {name}.</h2>
      <p>{canReport ? "Lihat perkembangan usaha dan lanjutkan pekerjaan Anda dari sini." : "Akses pekerjaan harian Anda melalui pintasan di bawah."}</p>
    </div>
    <dl className="dashboard-workspace">
      <div><dt><Icon name="store" size={16} />Cabang aktif</dt><dd>{branch?.name ?? "Belum dipilih"}</dd></div>
      <div><dt><Icon name="cart" size={16} />Terminal</dt><dd>{register?.name ?? "Belum dipilih"}</dd></div>
    </dl>
  </section>
  <OnboardingChecklist />
  <div className="dashboard-period"><h2>Ringkasan 7 hari</h2><span><Icon name="clock" size={15} />{period}</span></div>
  <div className="stat-grid">{cards.map(([label, value, icon, note]) => <section className="stat-card" key={label}>
      <div>
        <p>{label}</p>
        <strong>{value == null ? "—" : label === "Transaksi" ? value.toString() : formatIDR(value)}</strong>
        <span>{!branch ? "Pilih cabang aktif" : !canReport ? "Akses laporan dibatasi" : sales.loading ? "Memuat ringkasan…" : sales.error ? "Data belum dapat dimuat" : note}</span>
      </div>
      <span className="stat-icon"><Icon name={icon} size={23} /></span>
    </section>)}</div>
  <nav className="dashboard-shortcuts" aria-label="Akses cepat">{quickLinks.map(([to, label, description, icon]) => <Link key={to} to={to}><span className="dashboard-shortcut-icon"><Icon name={icon} size={21} /></span><span><strong>{label}</strong><small>{description}</small></span><Icon name="arrow" size={16} /></Link>)}</nav>
  <div className="dashboard-report-grid">
    <section className="panel chart-panel">
      <div className="panel-heading">
        <div>
          <h2>Tren penjualan</h2>
          <p className="muted small">7 hari terakhir · {branch?.name ?? "Belum ada cabang"}</p>
        </div>
        <span className="dashboard-panel-icon"><Icon name="chart" /></span>
      </div>
      {!enabled ? <div className="empty-state">
        <Icon name="chart" size={32} />
        <h3>{canReport ? "Pilih cabang untuk memulai" : "Ringkasan untuk pemilik & manajer"}</h3>
        <p>{canReport ? "Data penjualan muncul setelah cabang aktif dipilih." : "Gunakan menu sesuai akses akun Anda."}</p>
      </div> : <ResourceState resource={sales} empty={!rows.length} message="Transaksi yang selesai akan muncul di ringkasan ini.">
        <div className="chart-bars" role="img" aria-label={days.map(d => `${d.day}: ${d.total == null ? "data belum lengkap" : formatIDR(d.total)}`).join("; ")}>{days.map(d => <div className="chart-column" key={d.day}>
            <span className="chart-tooltip">{d.total == null ? "Belum lengkap" : formatIDR(d.total)}</span>
            <div className="chart-track"><div className="chart-bar" style={{
                  height: d.total != null && max > 0n ? `${Number((d.total > 0n ? d.total : 0n) * 100n / max)}%` : "0%"
                }} /></div>
            <span>{d.label}<small>{d.day.slice(8)}</small></span>
          </div>)}</div>
        {stats?.net == null && <p className="report-note">Histori retur belum lengkap; penjualan neto belum dapat dihitung.</p>}
        <div className="chart-caption"><span className="legend-mark" />Penjualan neto<Link to="/laporan">Buka laporan<Icon name="arrow" size={15} /></Link></div>
      </ResourceState>}
    </section>
    <section className="panel">
      <div className="panel-heading">
        <div>
          <h2>Komposisi pembayaran</h2>
          <p className="muted small">Penerimaan per metode · 7 hari</p>
        </div>
        <span className="dashboard-panel-icon"><Icon name="wallet" /></span>
      </div>
      {enabled ? <ResourceState resource={payments} empty={!payments.data?.rows?.length} message="Metode pembayaran tampil setelah ada transaksi."><div className="payment-list">{Object.entries((payments.data?.rows ?? []).reduce((acc, row) => {
              const label = `${row.method} · ${row.verification === "SYSTEM" ? "Terverifikasi" : "Manual"}`;
              acc[label] = (acc[label] ?? 0n) + BigInt(row.net_cents);
              return acc;
            }, {})).map(([label, value]) => <div key={label}>
            <span className="payment-method-icon"><Icon name="wallet" /></span>
            <div>
              <strong>{label}</strong>
              <span>Penerimaan neto</span>
            </div>
            <b>{formatIDR(value)}</b>
          </div>)}</div></ResourceState> : <div className="empty-state">{canReport ? "Pilih cabang untuk melihat pembayaran." : "Laporan dibatasi sesuai peran."}</div>}
    </section>
  </div>
  <section className="panel products-summary">
    <div className="panel-heading">
      <div>
        <h2>Produk dengan penjualan tertinggi</h2>
        <p className="muted small">5 produk teratas berdasarkan penjualan neto · 7 hari terakhir.</p>
      </div>
      {canReport && <Link to="/laporan" className="text-link">Lihat laporan<Icon name="arrow" size={16} /></Link>}
    </div>
    {enabled ? <ResourceState resource={products} empty={!products.data?.rows?.length} message="Produk yang terjual akan ditampilkan di sini."><div className="table-scroll"><table>
          <thead><tr>
              <th>Produk</th>
              <th className="num">Terjual (satuan dasar)</th>
              <th className="num">Penjualan neto</th>
            </tr></thead>
          <tbody>{[...(products.data?.rows ?? [])].sort((a, b) => {
                const x = BigInt(a.net_sales_cents ?? 0);
                const y = BigInt(b.net_sales_cents ?? 0);
                return x > y ? -1 : x < y ? 1 : 0;
              }).slice(0, 5).map((row, i) => <tr key={i}>
              <td><span className="product-name">
                  <span className="product-avatar" aria-label={`Peringkat ${i + 1}`}>{String(i + 1).padStart(2, '0')}</span>
                  {row.product_name}
                </span></td>
              <td className="num">{formatQty(BigInt(row.sold_base_qty_milli))}</td>
              <td className="num">{row.net_sales_cents == null ? "Belum lengkap" : formatIDR(BigInt(row.net_sales_cents))}</td>
            </tr>)}</tbody>
        </table></div></ResourceState> : <div className="empty-state">{canReport ? "Pilih cabang untuk melihat performa produk." : "Laporan dibatasi sesuai peran."}</div>}
  </section>
</div>;
}
