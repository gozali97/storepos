import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { formatQty } from '../money.js';
import { modifierText, statusLabel } from '../kds.js';

export function KitchenTicketPrint({ ticket, onClose }) {
  if (!ticket) return null;
  return <TicketDialog key={ticket.ticket_id} ticket={ticket} onClose={onClose} />;
}

function TicketDialog({ ticket, onClose }) {
  const dialog = useRef(null);
  const [width, setWidth] = useState('80');
  useEffect(() => {
    const node = dialog.current;
    const previous = document.activeElement;
    node.showModal();
    return () => { node.close(); if (previous?.isConnected) previous.focus(); };
  }, []);
  return createPortal(<dialog ref={dialog} className="kitchen-print panel" onClose={onClose} aria-labelledby="kitchen-print-title">
    <div className="no-print table-toolbar"><h2 id="kitchen-print-title">Pratinjau tiket dapur</h2><div className="row"><button onClick={() => window.print()}>Cetak</button><label>Kertas<select value={width} onChange={event => setWidth(event.target.value)}><option value="80">80 mm</option><option value="58">58 mm</option></select></label><button onClick={() => dialog.current.close()}>Tutup</button></div></div>
    <div className="kitchen-paper" style={{ width: `${width}mm` }}>
      <strong>{ticket.label}</strong>
      <h2>{ticket.table_name ?? ticket.order_type}</h2>
      <p>{ticket.station_name ?? 'Stasiun lama (nama tidak tersedia)'} · {new Date(ticket.created_at).toLocaleString('id-ID')}</p>
      <hr />
      {ticket.items.map(item => { const s = item.snapshot ?? {}; const mods = modifierText(s.modifiers); return <div key={item.line_id} className={item.status === 'CANCELLED' ? 'cancelled' : ''}>
        <strong>{formatQty(BigInt(s.qty_milli ?? 0))} × {s.product_name}</strong>
        {s.variant_name && <div>{s.variant_name}</div>}{mods && <div>+ {mods}</div>}{s.note && <div>Catatan: {s.note}</div>}
        <small>{statusLabel(item.status)}{item.cancel_reason ? ` — ${item.cancel_reason}` : ''}</small><hr />
      </div>; })}
      <small>ID tiket: {ticket.ticket_id}</small>
    </div>
  </dialog>, document.body);
}
