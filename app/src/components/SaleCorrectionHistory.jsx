import { useState } from 'react';
import { useResource } from '../hooks/useResource.js';
import { formatIDR } from '../money.js';
import { ResourceState } from './UI.jsx';

export function SaleCorrectionHistory({ saleId, timezone }) {
  return <History key={saleId} saleId={saleId} timezone={timezone} />;
}
function History({ saleId, timezone }) {
  const [cursor, setCursor] = useState('');
  const returns = useResource(`/sales/${saleId}/returns${cursor ? `?after=${cursor}` : ''}`);
  const reversal = useResource(`/sales/${saleId}/void`);
  const time = value => value ? new Date(value).toLocaleString('id-ID', { timeZone: timezone ?? 'Asia/Jakarta' }) : '—';
  return <section><h3>Riwayat retur dan void</h3>
    <ResourceState resource={reversal}>{reversal.data?.status === 'voided' && <div className="alert warn"><strong>Transaksi void</strong><p>{reversal.data.reason}</p><small>{time(reversal.data.voided_at)} · pelaku {reversal.data.voided_by ?? 'Tidak tersedia'} · penyetuju {reversal.data.approved_by ?? 'Tidak tersedia'}</small></div>}</ResourceState>
    <ResourceState resource={returns} empty={!returns.data?.returns?.length} message="Belum ada dokumen retur."><div className="table-scroll"><table><thead><tr><th>Dokumen/waktu</th><th>Refund</th><th>Pembulatan</th><th>Alasan</th><th>Pelaku/penyetuju</th></tr></thead><tbody>{returns.data?.returns?.map(r => <tr key={r.return_id}><td>{r.return_id}<small className="table-subtext">{time(r.created_at)}</small></td><td>{formatIDR(BigInt(r.amount_cents))}</td><td>{r.cash_rounding_cents == null ? 'Historis tidak lengkap' : formatIDR(BigInt(r.cash_rounding_cents))}</td><td>{r.reason}</td><td>{r.created_by} / {r.approved_by ?? 'Tidak ada'}</td></tr>)}</tbody></table></div></ResourceState>
    <div className="row"><button disabled={!cursor || returns.loading} onClick={() => setCursor('')}>Retur halaman pertama</button><button disabled={returns.loading || !returns.data?.next_cursor} onClick={() => setCursor(returns.data.next_cursor)}>Retur berikutnya</button><button onClick={() => { returns.reload(); reversal.reload(); }}>Muat ulang koreksi</button></div>
    <p className="muted small">Refund nontunai diselesaikan manual. Histori ini mencatat dokumen koreksi, bukan konfirmasi dana dari penyedia pembayaran.</p>
  </section>;
}
