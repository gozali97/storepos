import { normalizeCart } from './cashier.js';

export function offlineCart(snapshot) {
  return { base: snapshot.active ?? null, cart: normalizeCart(snapshot.active?.cart), catalog: snapshot.catalog ?? [], productUnits: snapshot.productUnits ?? {} };
}

export async function syncOfflineCart(client, data, branchID, registerID) {
  const response = await client.get(`/drafts?register_id=${encodeURIComponent(registerID)}`);
  const active = (response.drafts ?? []).find(draft => draft.status === 'active');
  if (data.base?.id) {
    if (active?.id === data.base.id && JSON.stringify(normalizeCart(active.cart)) === JSON.stringify(normalizeCart(data.cart))) return active;
    if (!active || active.id !== data.base.id || active.version !== data.base.version) throw new Error('Draft server telah berubah. Keranjang offline tetap disimpan; selesaikan konflik sebelum menggantinya.');
    return client.put(`/drafts/${active.id}`, { status: 'active', cart: data.cart }, { version: active.version });
  }
  if (active) {
    if (JSON.stringify(normalizeCart(active.cart)) === JSON.stringify(normalizeCart(data.cart))) return active;
    throw new Error('Terminal sudah memiliki draft aktif. Keranjang offline tetap disimpan.');
  }
  return client.post('/drafts', { branch_id: branchID, register_id: registerID, cart: data.cart });
}
