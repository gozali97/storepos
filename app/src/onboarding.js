// Logika murni checklist onboarding (CORE-001, §8.1). Status selesai dihitung server dari data nyata.

const STEPS = {
  branch_address: { label: 'Lengkapi alamat cabang', to: '/pengaturan' },
  first_product: { label: 'Tambah produk pertama', to: '/produk' },
  invite_team: { label: 'Undang anggota tim (opsional)', to: '/anggota' },
  open_shift: { label: 'Buka shift', to: '/kasir' },
  first_sale: { label: 'Lakukan transaksi pertama', to: '/kasir' },
};

export function stepInfo(key) {
  return STEPS[key] ?? { label: key, to: '/' };
}

/** Panel tampil selama belum disembunyikan dan masih ada langkah tersisa. */
export function shouldShowChecklist(data) {
  return !!data && !data.dismissed && data.done < data.total;
}

export function progressPercent(done, total) {
  if (!total || total <= 0) return 0;
  return Math.min(100, Math.round((done / total) * 100));
}

/** Langkah belum selesai pertama menjadi anjuran berikutnya. */
export function nextStep(items = []) {
  return items.find(item => !item.done) ?? null;
}

/** Teks metrik M2 (menit dari bisnis dibuat sampai penjualan pertama); kosong sebelum ada penjualan. */
export function m2Label(minutes) {
  if (minutes == null) return '';
  if (minutes < 1) return 'Transaksi pertama kurang dari 1 menit setelah bisnis dibuat';
  if (minutes < 60) return `Transaksi pertama ${minutes} menit setelah bisnis dibuat`;
  const hours = Math.floor(minutes / 60);
  return `Transaksi pertama ${hours} jam ${minutes % 60} menit setelah bisnis dibuat`;
}

export function onboardingEventBody(step, status) {
  if (!['business', 'type', 'branch', 'settings', 'product'].includes(step)) throw new Error('Langkah tidak valid.');
  if (!['completed', 'skipped'].includes(status)) throw new Error('Status tidak valid.');
  return { step, status };
}
