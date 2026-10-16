import { useEffect, useMemo, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { createApiClient } from '../api/client.js';
import { ReceiptPaper } from '../components/Receipt.jsx';
import { ResourceState } from '../components/UI.jsx';

export default function PublicReceiptPage() {
  const { hash } = useLocation();
  const token = hash.slice(1);
  const client = useMemo(() => createApiClient(), []);
  const [state, setState] = useState({ data: null, error: null, loading: true });
  const [revision, setRevision] = useState(0);
  const [width, setWidth] = useState('80');
  useEffect(() => {
    const controller = new AbortController();
    setState({ data: null, error: null, loading: true });
    if (!token) { setState({ data: null, loading: false, error: { message: 'Tautan struk tidak lengkap.' } }); return () => controller.abort(); }
    client.post('/public/receipt', { token }, { signal: controller.signal }).then(data => {
      if (!controller.signal.aborted) setState({ data, error: null, loading: false });
    }).catch(error => {
      if (error.name !== 'AbortError') setState({ data: null, loading: false, error: { ...error, message: error.status === 404 ? 'Struk tidak ditemukan atau tautannya telah dicabut.' : error.message } });
    });
    return () => controller.abort();
  }, [token, client, revision]);
  return <main className="public-receipt"><section className="panel"><div className="receipt-controls no-print"><h1>Struk digital</h1><label>Lebar kertas<select value={width} onChange={event => setWidth(event.target.value)}><option value="58">58 mm</option><option value="80">80 mm</option></select></label><button className="primary" disabled={!state.data} onClick={() => window.print()}>Cetak struk</button></div><ResourceState resource={{ ...state, reload: () => setRevision(value => value + 1) }}>{state.data && <ReceiptPaper receipt={state.data} width={width} />}</ResourceState></section></main>;
}
