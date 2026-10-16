import { useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { useSession } from "../auth/SessionContext.jsx";
import { Icon } from "../components/Icon.jsx";
import { BrandLogo } from "../components/BrandLogo.jsx";
import { Notice } from "../components/UI.jsx";
export function LoginPage() {
  const {
    status,
    masuk
  } = useSession();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [visible, setVisible] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  if (status === "masuk") return <Navigate to="/dashboard" replace />;
  async function submit(event) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setMessage("");
    try {
      await masuk(email.trim(), password);
      navigate("/dashboard", {
        replace: true
      });
    } catch (error) {
      setMessage(error.status === 429 || error.code === "RATE_LIMITED" || error.code === "TOO_MANY_REQUESTS" ? "Terlalu banyak percobaan. Tunggu beberapa menit sebelum mencoba kembali." : error.status === 401 ? "Email atau kata sandi salah. Periksa kembali akun Anda." : error.message || "Server belum dapat dihubungi. Coba lagi.");
    } finally {
      setBusy(false);
    }
  }
  return <main className="login-page">
  <header className="login-header">
    <a className="login-brand" href="/masuk" aria-label="StorePOS"><BrandLogo variant="header" /></a>
    <span>Ruang kerja usaha Anda</span>
  </header>
  <div className="login-layout">
    <section className="login-form-section"><div className="login-form-wrap">
        <span className="login-kicker">STOREPOS · ADMIN</span>
        <h1>Selamat datang<br /><span className="blue-text">kembali.</span></h1>
        <p className="login-description">Masuk untuk mengelola penjualan, persediaan,<br className="desktop-only" /> dan operasional usaha Anda.</p>
        <form onSubmit={submit} className="login-form" aria-busy={busy}>
          <Notice>{message}</Notice>
          <div className="field">
            <label htmlFor="email">Email</label>
            <input id="email" type="email" placeholder="Alamat email Anda" autoComplete="username" required maxLength={254} disabled={busy} value={email} onChange={e => setEmail(e.target.value)} />
          </div>
          <div className="field">
            <label htmlFor="password">Kata sandi</label>
            <div className="password-field">
              <input id="password" type={visible ? "text" : "password"} placeholder="Masukkan kata sandi" autoComplete="current-password" required disabled={busy} value={password} onChange={e => setPassword(e.target.value)} />
              <button type="button" aria-label={visible ? "Sembunyikan kata sandi" : "Tampilkan kata sandi"} aria-pressed={visible} onClick={() => setVisible(v => !v)}><Icon name="eye" /></button>
            </div>
          </div>
          <p className="login-note"><Icon name="users" size={16} />Gunakan akun yang diundang ke bisnis Anda.</p>
          <button className="primary login-submit" disabled={busy}>
            {busy ? "Memproses…" : "Masuk ke akun"}
            <Icon name="arrow" size={18} />
          </button>
          <span className="visually-hidden" role="status">{busy ? "Sedang memeriksa akun." : ""}</span>
        </form>
        <p className="login-support">Belum memiliki akses? Hubungi pemilik bisnis Anda.</p>
      </div></section>
    <aside className="login-cover">
      <div className="cover-shape cover-shape-one" />
      <div className="cover-shape cover-shape-two" />
      <div className="cover-content">
        <span className="cover-logo-panel"><BrandLogo variant="primary" /></span>
        <h2>Satu tempat.<br />Seluruh operasional.</h2>
        <p>Mulai hari dengan catatan yang rapi.<br />Kelola usaha dengan alur yang sesuai.</p>
        <div className="cover-features">
          <span><Icon name="receipt" />Penjualan</span>
          <span><Icon name="box" />Persediaan</span>
          <span><Icon name="chart" />Laporan</span>
        </div>
      </div>
      <span className="cover-caption">UNTUK TOKO, KAFE, DAN APOTEK</span>
    </aside>
  </div>
  <footer className="login-footer">© {new Date().getFullYear()} StorePOS · <a href="/">Kembali ke beranda</a></footer>
</main>;
}
