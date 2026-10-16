export const CONSENT_VERSION = 'demo-2026-10-14';
export const PRIVACY_VERSION = 'privacy-2026-10-14';

export function normalizePhone(value) {
  const cleaned = value.trim().replace(/[\s().-]/g, '');
  let phone = cleaned;
  if (phone.startsWith('08')) phone = '+62' + phone.slice(1);
  else if (phone.startsWith('62')) phone = '+' + phone;
  return /^\+628\d{7,12}$/.test(phone) ? phone : '';
}

export function validateLead(data) {
  const errors = {};
  for (const [field, label, max] of [['name', 'Nama lengkap', 100], ['business_name', 'Nama usaha', 120], ['city', 'Kota/kabupaten', 80]]) {
    const size = [...(data[field] || '').trim()].length;
    if (size < 2 || size > max) errors[field] = `${label} harus 2–${max} karakter.`;
  }
  if (!normalizePhone(data.phone || '')) errors.phone = 'Gunakan nomor Indonesia yang diawali 08, 62, atau +62.';
  if (!['retail', 'restaurant', 'pharmacy', 'other'].includes(data.business_type)) errors.business_type = 'Pilih jenis usaha Anda.';
  if (data.email && (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(data.email) || data.email.length > 254)) errors.email = 'Masukkan alamat email yang valid.';
  if ([...(data.notes || '')].length > 1000) errors.notes = 'Catatan maksimal 1.000 karakter.';
  if (!data.consent) errors.consent = 'Persetujuan diperlukan untuk menghubungi Anda terkait demo.';
  return errors;
}
