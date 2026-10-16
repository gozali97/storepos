// Logika murni halaman apotek: ED, status batch, dan body request (APT-003, APT-007).

/** Jumlah unit bulat (satuan dasar) menjadi milli-unit string; pecahan ditolak. */
export function unitsToMilli(value) {
  const text = String(value ?? '').trim();
  if (!/^\d+$/.test(text) || BigInt(text) <= 0n) throw new Error('Jumlah harus bilangan bulat lebih dari nol (satuan dasar).');
  return (BigInt(text) * 1000n).toString();
}

/** Label ED untuk daftar dan kasir: kedaluwarsa, mendekati, atau aman. */
export function expiryLabel(daysToExpiry, warnDays = 90) {
  if (daysToExpiry <= 0) return { tone: 'danger', text: 'Kedaluwarsa' };
  if (daysToExpiry <= warnDays) return { tone: 'warn', text: `ED < ${warnDays} hari (${daysToExpiry} hari lagi)` };
  return { tone: 'ok', text: `${daysToExpiry} hari lagi` };
}

/** Menerima YYYY-MM-DD atau YYYY-MM; format lain ditolak sebelum dikirim ke server. */
export function normalizeExpiry(value) {
  const text = String(value ?? '').trim();
  if (!/^\d{4}-(0[1-9]|1[0-2])(-(0[1-9]|[12]\d|3[01]))?$/.test(text)) throw new Error('ED harus berformat YYYY-MM-DD atau YYYY-MM.');
  return text;
}

export function receiptBody(branchID, v) {
  const batch = String(v.batch_number ?? '').trim();
  if (!batch) throw new Error('Nomor batch wajib diisi.');
  if (!v.variant_id || !v.supplier_id) throw new Error('Pilih obat dan supplier.');
  return {
    branch_id: branchID,
    variant_id: v.variant_id,
    supplier_id: v.supplier_id,
    batch_number: batch,
    expiry_date: normalizeExpiry(v.expiry_date),
    invoice_number: String(v.invoice_number ?? '').trim(),
    qty_milli: unitsToMilli(v.qty),
    unit_cost_cents: v.unit_cost_cents,
    status: v.status === 'QUARANTINE' ? 'QUARANTINE' : 'SELLABLE',
  };
}

/** Field tambahan finalisasi untuk apotek; referensi resep dan verifikasi apoteker opsional. */
export function pharmacyFinalizeFields(prescriptionRef, pharmacistEmail, pharmacistPin) {
  const ref = String(prescriptionRef ?? '').trim();
  const email = String(pharmacistEmail ?? '').trim();
  const pin = String(pharmacistPin ?? '').trim();
  const out = {};
  if (ref) out.prescription_reference = ref;
  if (email || pin) {
    if (!email || !/^\d{6}$/.test(pin)) throw new Error('Email apoteker dan PIN 6 digit harus diisi bersama.');
    out.pharmacist_verification = { email, pin };
  }
  return out;
}

/** Kuantitas baris dalam satuan dasar (milli); null bila pecahan karena apotek tidak menjual pecahan. */
export function baseQtyMilli(qtyMilli, factorMilli = '1000') {
  const base = (BigInt(qtyMilli) * BigInt(factorMilli)) / 1000n;
  return base > 0n && base % 1000n === 0n ? base.toString() : null;
}

/** Pesan kasir untuk error khusus apotek; null bila bukan error apotek. */
export function pharmacyErrorMessage(code) {
  if (code === 'BATCH_EXPIRED') return 'Batch kedaluwarsa tidak dapat dijual. Hapus baris atau pilih batch lain.';
  if (code === 'PRESCRIPTION_REFERENCE_REQUIRED') return 'Obat ini wajib resep. Isi referensi resep dan verifikasi apoteker.';
  return null;
}

export function batchActionBody(branchId, action, status, qtyUnits, reason) {
  const why = String(reason ?? '').trim();
  if (!why) throw new Error('Alasan wajib diisi.');
  if ([...why].length > 200) throw new Error('Alasan maksimal 200 karakter.');
  const body = { branch_id: branchId, qty_milli: unitsToMilli(qtyUnits), reason: why };
  if (action === 'dispose') {
    if (!['SELLABLE', 'QUARANTINE'].includes(status)) throw new Error('Status sumber pemusnahan tidak valid.');
    body.status = status;
  }
  return body;
}

export function batchCountBody(branchId, status, countedUnits, reason) {
  const why = String(reason ?? '').trim();
  if (!branchId || !['SELLABLE', 'QUARANTINE'].includes(status)) throw new Error('Cabang/status opname tidak valid.');
  if (!/^\d+$/.test(String(countedUnits).trim())) throw new Error('Hasil hitung harus bilangan bulat tidak negatif.');
  if (!why) throw new Error('Alasan opname wajib diisi.');
  if ([...why].length > 200) throw new Error('Alasan maksimal 200 karakter.');
  return { branch_id: branchId, status, counted_milli: (BigInt(String(countedUnits).trim()) * 1000n).toString(), reason: why };
}

export function batchReportPath(branchId, from, to, filter = 'all', days = 90, format = 'json') {
  const q = new URLSearchParams({ branch_id: branchId, from, to, filter, days: String(days) });
  if (format === 'csv') q.set('format', 'csv');
  return `/reports/stock/batches?${q}`;
}
