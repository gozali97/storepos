import { lazy, Suspense } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { useSession } from './auth/SessionContext.jsx';
import { LoginPage } from './pages/LoginPage.jsx';
import { NotFoundPage } from './pages/NotFoundPage.jsx';
import { AdminLayout } from './components/AdminLayout.jsx';
import { BrandLogo } from './components/BrandLogo.jsx';
import { WorkspaceProvider } from './workspace/WorkspaceContext.jsx';

const HomePage = lazy(() => import('./pages/HomePage.jsx'));
const OnboardingPage = lazy(() => import('./pages/OnboardingPage.jsx'));
const GoodsReceiptsPage = lazy(() => import('./pages/GoodsReceiptsPage.jsx'));
const ProductsPage = lazy(() => import('./pages/ProductsPage.jsx'));
const CatalogImportPage = lazy(() => import('./pages/CatalogImportPage.jsx'));
const ContactsPage = lazy(() => import('./pages/ContactsPage.jsx'));
const ReportsPage = lazy(() => import('./pages/ReportsPage.jsx'));
const RestaurantReportsPage = lazy(() => import('./pages/RestaurantReportsPage.jsx'));
const ReconciliationPage = lazy(() => import('./pages/ReconciliationPage.jsx'));
const SettingsPage = lazy(() => import('./pages/SettingsPage.jsx'));
const SecurityPage = lazy(() => import('./pages/SecurityPage.jsx'));
const ExportHistoryPage = lazy(() => import('./pages/ExportHistoryPage.jsx'));
const TaxSettingsPage = lazy(() => import('./pages/TaxSettingsPage.jsx'));
const ShiftHistoryPage = lazy(() => import('./pages/ShiftHistoryPage.jsx'));
const ExpensesPage = lazy(() => import('./pages/ExpensesPage.jsx'));
const AuditPage = lazy(() => import('./pages/AuditPage.jsx'));
const CashierPage = lazy(() => import('./pages/CashierPage.jsx'));
const MembersPage = lazy(() => import('./pages/MembersPage.jsx'));
const PharmacyPage = lazy(() => import('./pages/PharmacyPage.jsx'));
const PharmacyTools = lazy(() => import('./pages/PharmacyTools.jsx'));
const MenuAvailabilityPage = lazy(() => import('./pages/MenuAvailabilityPage.jsx'));
const OrderPage = lazy(() => import('./pages/OrderPage.jsx'));
const RestaurantOrdersPage = lazy(() => import('./pages/RestaurantOrdersPage.jsx'));
const TableManagementPage = lazy(() => import('./pages/TableManagementPage.jsx'));
const TablesPage = lazy(() => import('./pages/TablesPage.jsx'));
const KitchenPage = lazy(() => import('./pages/KitchenPage.jsx'));
const SalesPage = lazy(() => import('./pages/SalesPage.jsx'));
const PublicReceiptPage = lazy(() => import('./pages/PublicReceiptPage.jsx'));

export function App() {
  const { status, error, muatUlang, businessId } = useSession();
  const location = useLocation();
  if (location.pathname === '/receipt') return <Suspense fallback={<main className="boot" role="status">Memuat struk…</main>}><PublicReceiptPage /></Suspense>;
  if (status === 'memuat') {
    return <main className="boot"><div className="boot-content" role="status">
      <BrandLogo className="boot-logo" /><span className="loading-ring" /><strong>StorePOS</strong><p>Memuat ruang kerja Anda…</p>
    </div></main>;
  }
  if (status === 'gagal') {
    return <main className="boot"><section className="panel boot-content">
      <h1>Koneksi belum tersedia</h1><p role="alert">{error?.message}</p>
      <button className="primary" onClick={muatUlang}>Coba lagi</button>
    </section></main>;
  }
  return (
    <Routes>
      <Route path="/masuk" element={<LoginPage />} />
      <Route element={status === 'masuk'
        ? <WorkspaceProvider key={businessId}><AdminLayout /></WorkspaceProvider>
        : <Navigate to="/masuk" replace />}>
        <Route path="dashboard" element={<HomePage />} />
        <Route path="mulai" element={<OnboardingPage key={businessId} />} />
        <Route path="penerimaan" element={<GoodsReceiptsPage key={businessId} />} />
        <Route path="produk" element={<ProductsPage key={businessId} />} />
        <Route path="impor-katalog" element={<CatalogImportPage key={businessId} />} />
        <Route path="kasir" element={<CashierPage key={businessId} />} />
        <Route path="supplier" element={<ContactsPage key="suppliers" kind="suppliers" />} />
        <Route path="pelanggan" element={<ContactsPage key="customers" kind="customers" />} />
        <Route path="laporan" element={<ReportsPage key={`reports-${businessId}`} />} />
        <Route path="restoran/laporan" element={<RestaurantReportsPage key={businessId} />} />
        <Route path="rekonsiliasi" element={<ReconciliationPage key={businessId} />} />
        <Route path="stok" element={<ReportsPage key={`stock-${businessId}`} stockOnly />} />
        <Route path="pengeluaran" element={<ExpensesPage key={businessId} />} />
        <Route path="pengaturan" element={<SettingsPage key={businessId} />} />
        <Route path="keamanan" element={<SecurityPage />} />
        <Route path="ekspor" element={<ExportHistoryPage key={businessId} />} />
        <Route path="pajak" element={<TaxSettingsPage key={businessId} />} />
        <Route path="shift" element={<ShiftHistoryPage key={businessId} />} />
        <Route path="audit" element={<AuditPage />} />
        <Route path="anggota" element={<MembersPage key={businessId} />} />
        <Route path="batch" element={<PharmacyPage key={businessId} />} />
        <Route path="apotek/kelola" element={<PharmacyTools key={businessId} />} />
        <Route path="pesanan" element={<RestaurantOrdersPage key={businessId} />} />
        <Route path="pesanan/:id" element={<OrderPage key={businessId} />} />
        <Route path="menu-habis" element={<MenuAvailabilityPage key={businessId} />} />
        <Route path="meja/kelola" element={<TableManagementPage key={businessId} />} />
        <Route path="meja" element={<TablesPage key={businessId} />} />
        <Route path="dapur" element={<KitchenPage key={businessId} />} />
        <Route path="transaksi" element={<SalesPage key={businessId} />} />
      </Route>
      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  );
}
