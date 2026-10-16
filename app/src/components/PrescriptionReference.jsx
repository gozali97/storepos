import { useEffect, useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';
import { Notice } from './UI.jsx';

export function PrescriptionReference({ saleId }) {
  const { business, businessId, user } = useSession();
  if (business?.business_type !== 'pharmacy' || business?.role !== 'pharmacist') return null;
  return <PrivateReference key={`${businessId}.${user?.id}.${saleId}`} saleId={saleId} />;
}
function PrivateReference({ saleId }) {
  const { client } = useSession();
  const [open, setOpen] = useState(false);
  return <section className="no-print"><h3>Referensi resep terbatas</h3><p>Hanya Apoteker berizin cabang; setiap pembacaan dicatat audit. Jangan salin ke struk atau catatan umum.</p>{!open ? <button onClick={() => setOpen(true)}>Buka referensi (diaudit)</button> : <ReferenceData client={client} saleId={saleId} onClose={() => setOpen(false)} />}</section>;
}
function ReferenceData({ client, saleId, onClose }) {
  const [state, setState] = useState({ loading: true });
  useEffect(() => {
    const controller = new AbortController(); let active = true;
    client.get(`/pharmacy/sales/${saleId}/prescription`, { signal: controller.signal }).then(
      data => { if (active) setState({ data }); },
      error => { if (active && error.name !== 'AbortError') setState({ error }); },
    );
    return () => { active = false; controller.abort(); };
  }, [client, saleId]);
  useEffect(() => {
    const hide = () => { if (document.visibilityState !== 'visible') onClose(); };
    const print = () => onClose();
    document.addEventListener('visibilitychange', hide); window.addEventListener('beforeprint', print);
    return () => { document.removeEventListener('visibilitychange', hide); window.removeEventListener('beforeprint', print); };
  }, [onClose]);
  return <div className="alert warn" role="status">{state.loading && <p>Memuat akses terbatas…</p>}<Notice>{state.error?.message}</Notice>{state.data && <><p style={{ whiteSpace: 'pre-wrap' }}>{state.data.prescription_reference ?? 'Tidak ada referensi resep tercatat.'}</p><small>Pemverifikasi: {state.data.verified_by ?? 'Tidak tercatat'}</small></>}<button onClick={onClose}>Tutup dan hapus dari tampilan</button></div>;
}
