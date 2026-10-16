import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useResource } from '../hooks/useResource.js';
import { useSession } from '../auth/SessionContext.jsx';
import { useWorkspace } from '../workspace/WorkspaceContext.jsx';

export function LowStockNotice() {
  const { business } = useSession();
  const { branch } = useWorkspace();
  const allowed = ['owner', 'manager', 'warehouse', 'cashier', 'pharmacist'].includes(business?.role);
  // Backend memilih saldo batch layak jual untuk apotek dan saldo tersedia inti untuk retail/restoran.
  const enabled = allowed && branch;
  const resource = useResource(enabled ? `/stock/reorder?branch_id=${branch.id}` : null);
  useEffect(() => {
    if (!enabled) return undefined;
    const timer = setInterval(resource.reload, 60000);
    return () => clearInterval(timer);
  }, [!!enabled, branch?.id, resource.reload]);
  if (!enabled) return null;
  if (resource.error) return <div className="alert warn" role="status">Pemeriksaan stok minimum belum tersedia. <button onClick={resource.reload}>Coba lagi</button></div>;
  const count = resource.data?.items?.length ?? 0;
  if (!count) return null;
  return <div className="alert warn" role="status">{resource.data.next_cursor ? 'Lebih dari ' : ''}{count} produk mencapai stok minimum pada cabang {branch.name}. <Link to={business?.business_type === 'pharmacy' ? '/apotek/kelola' : '/stok'}>Periksa kebutuhan pengadaan</Link></div>;
}
