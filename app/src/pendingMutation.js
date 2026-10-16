// Simpan hanya aksi stok/pembayaran tanpa kredensial; retry harus identik sampai hasil pasti.
export function loadPendingMutation(storage, storageKey) {
  const raw = storage.getItem(storageKey);
  if (raw === null) return null;
  const op = JSON.parse(raw);
  if (!op || typeof op.path !== 'string' || !op.path.startsWith('/') || typeof op.key !== 'string' || !op.key) {
    throw new Error('Data pemulihan tidak valid. Jangan kirim aksi baru sebelum status diperiksa.');
  }
  if (!Number.isFinite(op.createdAt) || Date.now() - op.createdAt >= 72 * 60 * 60 * 1000 || op.createdAt > Date.now() + 60000) {
    throw new Error('Aksi pemulihan melewati batas 72 jam atau waktunya tidak valid. Periksa status server sebelum melanjutkan.');
  }
  return op;
}

export function isUncertainMutation(error) {
  return !error.status || error.status >= 500 || error.status === 408 || error.status === 429 || error.code === 'IDEMPOTENCY_CONFLICT';
}

export async function performPendingMutation(client, storage, storageKey, operation) {
  // JSON snapshot memastikan payload retry tidak berubah oleh edit form berikutnya.
  const snapshot = JSON.parse(JSON.stringify({ ...operation, createdAt: operation.createdAt ?? Date.now() }));
  storage.setItem(storageKey, JSON.stringify(snapshot)); // Gagal menyimpan: jangan kirim request.
  loadPendingMutation(storage, storageKey); // Jangan replay key yang mungkin sudah dibersihkan backend.
  try {
    const result = await client.post(snapshot.path, snapshot.body, { version: snapshot.version, idempotencyKey: snapshot.key, ...(snapshot.rawBody !== undefined ? { rawBody: snapshot.rawBody } : {}) });
    storage.removeItem(storageKey);
    return result;
  } catch (error) {
    if (!isUncertainMutation(error)) storage.removeItem(storageKey);
    throw error;
  }
}
