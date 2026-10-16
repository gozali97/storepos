import { useResource } from '../hooks/useResource.js';
import { expiryLabel } from '../pharmacy.js';
import { formatQty } from '../money.js';

// Pratinjau alokasi FEFO per baris keranjang (APT-005, APT-007). Final dihitung ulang server saat bayar.
export function FefoPreview({ branchId, variantId, baseQty }) {
  const path = branchId && variantId && baseQty
    ? `/pharmacy/fefo-preview?branch_id=${branchId}&variant_id=${variantId}&qty_milli=${baseQty}` : null;
  const { data, error, loading } = useResource(path);
  if (!path) return <small className="muted">Apotek tidak menjual pecahan satuan dasar.</small>;
  if (loading) return <small className="muted" role="status">Menghitung batch…</small>;
  if (error) return <small role="alert">Batch belum dapat dimuat: {error.message}</small>;
  const short = BigInt(data.shortage_milli) > 0n;
  return <ul className="fefo-preview" aria-label="Pratinjau alokasi batch">
    {data.allocations.map(a => {
      const label = expiryLabel(a.days_to_expiry);
      return <li key={a.batch_id}>Batch {a.batch_number} · ED {a.expiry_date} · {formatQty(BigInt(a.qty_milli))} <span className={`badge ${label.tone}`}>{label.text}</span></li>;
    })}
    {short && <li role="alert">Stok batch kurang {formatQty(BigInt(data.shortage_milli))}</li>}
  </ul>;
}
