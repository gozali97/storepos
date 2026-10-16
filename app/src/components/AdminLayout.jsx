import { Suspense, useEffect, useRef, useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { useSession } from "../auth/SessionContext.jsx";
import { useWorkspace } from "../workspace/WorkspaceContext.jsx";
import { termsFor, BUSINESS_TYPE_LABELS } from "../terms.js";
import { Icon } from "./Icon.jsx";
import { BrandLogo } from "./BrandLogo.jsx";
import { Notice } from "./UI.jsx";
import { LowStockNotice } from './LowStockNotice.jsx';
const roles = {
  owner: "Pemilik",
  manager: "Manajer",
  cashier: "Kasir",
  warehouse: "Gudang",
  pharmacist: "Apoteker",
  kitchen: "Dapur"
};
export function AdminLayout() {
  const {
    user,
    business,
    memberships,
    gantiBisnis,
    keluar,
    clockOffsetMs
  } = useSession();
  const workspace = useWorkspace();
  const location = useLocation();
  const [online, setOnline] = useState(navigator.onLine);
  const [message, setMessage] = useState("");
  const [leaving, setLeaving] = useState(false);
  const menu = useRef(null);
  const trigger = useRef(null);
  const terms = termsFor(business?.business_type);
  const role = business?.role;
  const reporting = ["owner", "manager", "pharmacist"].includes(role);
  const managing = ["owner", "manager"].includes(role);
  const selling = ["owner", "manager", "cashier", "pharmacist"].includes(role);
  const nav = [{
    to: "/dashboard",
    label: "Dashboard",
    icon: "dashboard"
  }, ...(selling ? [{
    to: "/kasir",
    label: "Kasir",
    icon: "cart"
  }, { to: '/transaksi', label: 'Riwayat transaksi', icon: 'receipt' }, { to: '/shift', label: 'Riwayat shift', icon: 'clock' }] : []), {
    to: "/produk",
    label: terms.products,
    icon: "box"
  }, {
    to: "/stok",
    label: "Persediaan",
    icon: "store"
  }, ...(business?.business_type === "restaurant" && ["owner", "manager", "cashier"].includes(role) ? [{
    to: "/meja",
    label: "Meja",
    icon: "store"
  }, {
    to: "/pesanan",
    label: "Daftar pesanan",
    icon: "receipt"
  }] : []), ...(business?.business_type === "restaurant" && ["owner", "manager"].includes(role) ? [{
    to: "/meja/kelola",
    label: "Kelola meja",
    icon: "settings"
  }] : []), ...(business?.business_type === "restaurant" && ["owner", "manager", "cashier"].includes(role) ? [{
    to: "/menu-habis",
    label: "Menu habis",
    icon: "box"
  }] : []), ...(business?.business_type === 'restaurant' && managing ? [{
    to: '/restoran/laporan',
    label: 'Operasional restoran',
    icon: 'chart'
  }] : []), ...(business?.business_type === "restaurant" && ["owner", "manager", "kitchen", "cashier"].includes(role) ? [{
    to: "/dapur",
    label: "Layar dapur",
    icon: "clock"
  }] : []), ...(business?.business_type === "pharmacy" && role !== "cashier" && role !== "kitchen" ? [{
    to: "/batch",
    label: "Batch & ED",
    icon: "clock"
  }, {
    to: "/apotek/kelola",
    label: "Laporan & identitas apotek",
    icon: "box"
  }] : []), ...(reporting ? [{
    to: "/laporan",
    label: "Laporan",
    icon: "chart"
  }, {
    to: "/ekspor",
    label: "Riwayat ekspor",
    icon: "receipt"
  }] : []), ...(managing || role === "warehouse" || role === 'pharmacist' ? [{
    to: "/penerimaan",
    label: "Dokumen penerimaan",
    icon: "truck"
  }] : []), ...(managing || role === "warehouse" ? [{
    to: "/supplier",
    label: "Supplier",
    icon: "truck"
  }] : []), ...(managing || ["cashier", "pharmacist"].includes(role) ? [{
    to: "/pelanggan",
    label: "Pelanggan",
    icon: "users"
  }] : [])];
  const accountNav = [...(role === 'owner' ? [{ to: '/pajak', label: 'Pajak & penugasan', icon: 'settings' }] : []), ...(managing ? [{ to: '/anggota', label: 'Anggota', icon: 'users' }] : []), ...(managing ? [{
    to: "/audit",
    label: "Log aktivitas",
    icon: "receipt"
  }, {
    to: '/rekonsiliasi',
    label: 'Rekonsiliasi',
    icon: 'chart'
  }] : []), ...(managing || role === 'cashier' ? [{
    to: "/pengeluaran",
    label: "Pengeluaran",
    icon: "wallet"
  }] : []), {
    to: "/keamanan",
    label: "Keamanan akun",
    icon: "users"
  }, {
    to: "/pengaturan",
    label: "Pengaturan",
    icon: "settings"
  }];
  const active = [...nav, ...accountNav].find(item => item.to === location.pathname);
  useEffect(() => {
    if (menu.current?.open) menu.current.close();
  }, [location.pathname]);
  useEffect(() => {
    const update = () => setOnline(navigator.onLine);
    window.addEventListener("online", update);
    window.addEventListener("offline", update);
    return () => {
      window.removeEventListener("online", update);
      window.removeEventListener("offline", update);
    };
  }, []);
  async function logout() {
    setLeaving(true);
    setMessage("");
    try {
      await keluar();
    } catch (error) {
      setMessage(error.message);
    } finally {
      setLeaving(false);
    }
  }
  const navigation = <>
  <NavLink className="brand" to="/dashboard"><span className="brand-symbol"><BrandLogo /></span>StorePOS</NavLink>
  <div className="sidebar-rule" />
  <nav aria-label="Menu utama">
    {nav.map(item => <NavLink end={item.to === "/dashboard" || item.to === "/meja"} key={item.to} to={item.to} className={({
        isActive
      }) => `nav-item ${isActive ? "active" : ""}`}>
      <span className="nav-icon"><Icon name={item.icon} /></span>
      {item.label}
    </NavLink>)}
    <p className="nav-section-title">AKUN & OPERASIONAL</p>
    {accountNav.map(item => <NavLink key={item.to} to={item.to} className={({
        isActive
      }) => `nav-item ${isActive ? "active" : ""}`}>
      <span className="nav-icon"><Icon name={item.icon} /></span>
      {item.label}
    </NavLink>)}
  </nav>
  <div className="sidebar-business">
    <Icon name="store" />
    <strong>{business?.business_name ?? "Bisnis Anda"}</strong>
    <span>{BUSINESS_TYPE_LABELS[business?.business_type] ?? "Pilih bisnis untuk memulai"}</span>
    <NavLink to="/pengaturan" className="button primary small-button">Pengaturan bisnis</NavLink>
  </div>
  <p className="sidebar-caption">StorePOS · Ruang kerja usaha</p>
</>;
  return <div className="admin-shell">
  <a href="#main-content" className="skip-link">Lewati ke konten</a>
  <aside className="sidebar">{navigation}</aside>
  <dialog ref={menu} className="mobile-menu" onClose={() => trigger.current?.focus()}>
    <button className="menu-close" aria-label="Tutup menu" onClick={() => menu.current.close()}><Icon name="close" /></button>
    {navigation}
  </dialog>
  <div className="admin-main">
    <header className="topbar">
      <div className="topbar-title">
        <button ref={trigger} className="mobile-menu-button" onClick={() => menu.current.showModal()} aria-label="Buka menu navigasi">
          <Icon name="menu" />
          <span>Menu</span>
        </button>
        <div>
          <div className="breadcrumb">
            <span>Halaman</span>
            <span>/</span>
            <span>{active?.label ?? "Halaman"}</span>
          </div>
          <strong>{active?.label ?? "StorePOS"}</strong>
        </div>
      </div>
      <div className="topbar-actions">
        <span className={`connection ${online ? "" : "offline"}`} role="status">{online ? "Online" : "Tanpa koneksi"}</span>
        <div className="account">
          <span className="avatar">{(user?.name ?? user?.email ?? "U").slice(0, 1).toUpperCase()}</span>
          <div>
            <strong>{user?.name ?? user?.email?.split("@")[0] ?? "Pengguna"}</strong>
            <span>{roles[role] ?? "Anggota"}</span>
          </div>
        </div>
        <button className="icon-button logout-button" disabled={leaving} aria-label="Keluar dari akun" onClick={logout}><Icon name="logout" /></button>
      </div>
    </header>
    <div className="workspace-bar">
      <label>Bisnis<select aria-label="Bisnis aktif" value={business?.business_id ?? ""} onChange={e => gantiBisnis(e.target.value)} disabled={!memberships.length}>{memberships.map(m => <option key={m.business_id} value={m.business_id}>{m.business_name}</option>)}</select></label>
      <label>Cabang<select aria-label="Cabang aktif" value={workspace.branch?.id ?? ""} onChange={e => workspace.selectBranch(e.target.value)} disabled={workspace.loading || !workspace.branches.length}>
          <option value="" disabled>{workspace.loading ? "Memuat cabang…" : "Belum ada cabang"}</option>
          {workspace.branches.map(b => <option key={b.id} value={b.id}>{b.name}</option>)}
        </select></label>
      <label>Terminal<select aria-label="Terminal aktif" value={workspace.register?.id ?? ""} onChange={e => workspace.selectRegister(e.target.value)} disabled={!workspace.registers.length}>
          <option value="" disabled>Belum ada terminal</option>
          {workspace.registers.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
        </select></label>
      <span className="workspace-zone">
        <Icon name="clock" size={16} />
        {workspace.branch?.timezone?.replace("Asia/", "") ?? "Zona waktu cabang"}
      </span>
    </div>
    <Notice>{message}</Notice>
    <LowStockNotice />
    {Math.abs(clockOffsetMs ?? 0) > 300000 && <div className="alert warn" role="alert">Jam perangkat {clockOffsetMs > 0 ? 'tertinggal' : 'mendahului'} waktu server sekitar {Math.round(Math.abs(clockOffsetMs) / 60000)} menit. Aktifkan pengaturan tanggal dan waktu otomatis pada perangkat.</div>}
    {workspace.error && <div className="alert error" role="alert">{workspace.error.message} <button onClick={workspace.reload}>Muat ulang cabang</button></div>}
    <main id="main-content" tabIndex={-1}><Suspense fallback={<div className="empty-state" role="status">Memuat halaman…</div>}><Outlet /></Suspense></main>
    <footer className="app-footer">
      <span>© {new Date().getFullYear()} StorePOS</span>
      <span>{business?.business_name ?? "Manajemen usaha Anda"}</span>
    </footer>
  </div>
</div>;
}
