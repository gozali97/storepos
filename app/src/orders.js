// Logika murni pesanan restoran: status kirim ke dapur, body request, dan validasi pembayaran (RST-003, RST-004, RST-009).
import { quantityToMilli, discountFromInput } from './cashier.js';
import { rupiahToCents } from './forms.js';

const LINE_LABELS = { NOT_SENT: 'Belum dikirim', SENT: 'Dikirim', IN_PROGRESS: 'Dimasak', READY: 'Siap', SERVED: 'Disajikan', CANCELLED: 'Dibatalkan' };

export function lineStatusLabel(status) {
  return LINE_LABELS[status] ?? status;
}

/** Jumlah baris yang belum dikirim ke dapur; pembayaran diblokir server selama masih ada. */
export function notSentCount(lines = []) {
  return lines.filter(line => line.kitchen_status === 'NOT_SENT').length;
}

/** Baris yang masih aktif (tidak dibatalkan) untuk ditampilkan dan dihitung. */
export function liveLines(lines = []) {
  return lines.filter(line => line.kitchen_status !== 'CANCELLED');
}

export function addLineBody(variantId, qty, note = '', modifierIds = []) {
  if (!variantId) throw new Error('Pilih menu terlebih dahulu.');
  const trimmed = String(note ?? '').trim();
  if ([...trimmed].length > 140) throw new Error('Catatan maksimal 140 karakter.');
  return {
    variant_id: variantId,
    qty_milli: quantityToMilli(qty),
    ...(trimmed ? { note: trimmed } : {}),
    ...(modifierIds.length ? { modifier_option_ids: [...modifierIds] } : {}),
  };
}

/**
 * Validasi pilihan modifier terhadap batas grup agar kesalahan terlihat sebelum request.
 * Server tetap menjadi penentu akhir.
 */
export function modifierError(groups = [], selected = []) {
  const chosen = new Set(selected);
  for (const group of groups) {
    const count = (group.options ?? []).filter(option => chosen.has(option.option_id)).length;
    if (count < group.min_choices) return `${group.name}: pilih minimal ${group.min_choices}.`;
    if (count > group.max_choices) return `${group.name}: pilih maksimal ${group.max_choices}.`;
  }
  return '';
}

/** Pembayaran tunggal sebesar tagihan; kembalian tunai dihitung server dari nominal yang diserahkan. */
export function checkoutPayments(method, amountCents) {
  if (!['cash', 'qris', 'card', 'transfer'].includes(method)) throw new Error('Metode pembayaran tidak dikenal.');
  if (!/^\d+$/.test(String(amountCents)) || BigInt(amountCents) <= 0n) throw new Error('Nominal pembayaran tidak valid.');
  return [{ method, amount_cents: String(amountCents) }];
}

/** Mengapa tombol bayar nonaktif; string kosong berarti boleh bayar. */
export function paymentBlockReason(bill) {
  if (!bill) return 'Memuat tagihan…';
  if (bill.not_sent_count > 0) return `${bill.not_sent_count} item belum dikirim ke dapur.`;
  if (bill.payment_blocked) return 'Pembayaran belum dapat diproses untuk pesanan ini.';
  return '';
}

const CANCEL_REASON_MAX = 200;

/**
 * Item yang sudah dikirim boleh dibatalkan; server menentukan perlu-tidaknya persetujuan atasan
 * (RST-008): lewat SENT atau peran tanpa izin penuh wajib PIN. UI hanya menampilkan petunjuk.
 */
export function cancelNeedsApproval(status, role) {
  if (status === 'NOT_SENT') return false;
  const fullPermission = role === 'owner' || role === 'manager';
  return status !== 'SENT' || !fullPermission;
}

export function cancelBody(reason, approvalEmail = '', approvalPin = '') {
  const text = String(reason ?? '').trim();
  if (!text) throw new Error('Alasan pembatalan wajib diisi.');
  if ([...text].length > CANCEL_REASON_MAX) throw new Error(`Alasan maksimal ${CANCEL_REASON_MAX} karakter.`);
  const email = String(approvalEmail ?? '').trim();
  const pin = String(approvalPin ?? '').trim();
  if (!email && !pin) return { reason: text };
  if (!email || !/^\d{6}$/.test(pin)) throw new Error('Email atasan dan PIN 6 digit harus diisi bersama.');
  return { reason: text, approval: { email, pin } };
}

/** Status "Habis" manual per menu dan cabang; version 0 berarti belum pernah diatur (RST-013). */
export function availabilityRequest(branchId, item, soldOut) {
  if (!branchId) throw new Error('Pilih cabang terlebih dahulu.');
  return { body: { branch_id: branchId, sold_out: !!soldOut }, version: item.version ?? 0 };
}

/** Teks stok tersedia untuk menu berlacak stok: saldo dikurangi cadangan dapur. */
export function availabilityLabel(item) {
  if (item.sold_out) return 'Habis (manual)';
  if (!item.track_stock) return 'Tersedia';
  return BigInt(item.available_milli) <= 0n ? 'Stok habis' : 'Tersedia';
}

export function splitCheckoutPayments(rows) {
  if (!Array.isArray(rows) || !rows.length) throw new Error('Isi minimal satu pembayaran.');
  return rows.map(row => checkoutPayments(row.method, rupiahToCents(row.amount))[0]);
}

export function editLineBody(qty, note = '') {
  const text = String(note ?? '').trim();
  if ([...text].length > 140) throw new Error('Catatan maksimal 140 karakter.');
  return { qty_milli: quantityToMilli(qty), note: text };
}

export function orderDiscountBody(kind, value, reason = '') {
  const text = String(reason ?? '').trim();
  if ([...text].length > 200) throw new Error('Alasan maksimal 200 karakter.');
  return { discount: discountFromInput(kind, value), reason: text };
}
