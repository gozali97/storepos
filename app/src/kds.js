// Logika murni layar dapur (KDS): status item, umur tiket, dan aksi yang diizinkan (RST-005, RST-006).

export const KITCHEN_FLOW = ['SENT', 'IN_PROGRESS', 'READY', 'SERVED'];

const LABELS = { SENT: 'Baru', IN_PROGRESS: 'Dimasak', READY: 'Siap', SERVED: 'Disajikan', CANCELLED: 'Dibatalkan' };
const ACTIONS = { SENT: 'Mulai masak', IN_PROGRESS: 'Tandai siap', READY: 'Sajikan' };

export function statusLabel(status) {
  return LABELS[status] ?? status;
}

/**
 * Aksi satu ketuk untuk maju satu langkah. SERVED hanya untuk peran saji (kasir/pelayan),
 * sama seperti aturan backend; item selesai atau dibatalkan tidak punya aksi.
 */
export function nextAction(status, { canPrepare, canServe }) {
  const index = KITCHEN_FLOW.indexOf(status);
  if (index < 0 || index === KITCHEN_FLOW.length - 1) return null;
  const next = KITCHEN_FLOW[index + 1];
  if (next === 'SERVED' ? !canServe : !canPrepare) return null;
  return { next, label: ACTIONS[status] };
}

/** Mundur satu langkah; backend mewajibkan alasan untuk mundur. */
export function previousStatus(status) {
  const index = KITCHEN_FLOW.indexOf(status);
  return index > 0 ? KITCHEN_FLOW[index - 1] : null;
}

/** Umur tiket dalam menit penuh, memakai waktu server agar jam perangkat dapur yang melenceng tidak berpengaruh. */
export function ticketAgeMinutes(createdAt, serverTime, fetchedAtMs, nowMs = Date.now()) {
  const base = Date.parse(serverTime) - Date.parse(createdAt);
  if (!Number.isFinite(base)) return 0;
  return Math.max(0, Math.floor((base + (nowMs - fetchedAtMs)) / 60000));
}

/** Warna umur: normal, perlu perhatian (>=10 menit), terlambat (>=20 menit). */
export function ageTone(minutes) {
  if (minutes >= 20) return 'danger';
  if (minutes >= 10) return 'warn';
  return 'ok';
}

/** Item yang masih relevan di layar: belum disajikan; item dibatalkan tetap tampil sebagai peringatan. */
export function activeItems(ticket) {
  return (ticket.items ?? []).filter(item => item.status !== 'SERVED');
}

export function kitchenOverview(tickets, filter = 'all') {
  const items = tickets.flatMap(activeItems);
  return {
    counts: Object.fromEntries(['SENT', 'IN_PROGRESS', 'READY'].map(status => [status, items.filter(item => item.status === status).length])),
    tickets: filter === 'all' ? tickets : tickets.filter(ticket => activeItems(ticket).some(item => item.status === filter)),
  };
}

export function modifierText(modifiers) {
  if (!Array.isArray(modifiers)) return '';
  return modifiers.map(m => m?.name ?? m?.modifier_name ?? '').filter(Boolean).join(', ');
}

/** Tiket dengan semua item aktif sudah READY ditandai siap diantar. */
export function ticketReady(ticket) {
  const live = activeItems(ticket).filter(item => item.status !== 'CANCELLED');
  return live.length > 0 && live.every(item => item.status === 'READY');
}

/** True bila perpindahan status adalah mundur; backend mewajibkan alasan untuk itu. */
export function isBackward(from, to) {
  const a = KITCHEN_FLOW.indexOf(from);
  const b = KITCHEN_FLOW.indexOf(to);
  return a >= 0 && b >= 0 && b < a;
}
