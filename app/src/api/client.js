// Klien API StorePOS.
// Tanggung jawab (PRD §16.3): kirim request terautentikasi, teruskan konteks bisnis
// aktif, kelola Idempotency-Key per percobaan aksi finansial/stok, dan memetakan
// error backend ke bentuk yang stabil untuk UI.

/** Error yang dilempar klien API. */
export class ApiError extends Error {
  constructor({ code, message, details, requestId, status }) {
    super(message || code || 'Terjadi kesalahan');
    this.name = 'ApiError';
    this.code = code;
    this.details = details || [];
    this.requestId = requestId;
    this.status = status;
  }
}

/** Kode error backend yang direspon khusus oleh UI. */
export const API_CODES = {
  STOCK_INSUFFICIENT: 'STOCK_INSUFFICIENT',
  SHIFT_NOT_OPEN: 'SHIFT_NOT_OPEN',
  BATCH_EXPIRED: 'BATCH_EXPIRED',
  PRESCRIPTION_REFERENCE_REQUIRED: 'PRESCRIPTION_REFERENCE_REQUIRED',
  MODULE_DISABLED: 'MODULE_DISABLED',
  IDEMPOTENCY_CONFLICT: 'IDEMPOTENCY_CONFLICT',
  PERMISSION_DENIED: 'PERMISSION_DENIED',
  VERSION_CONFLICT: 'VERSION_CONFLICT',
};

/**
 * Membuat klien API.
 * @param {{
 *   baseUrl?: string,
 *   getBusinessId?: () => string | null,
 *   getCsrfToken?: () => string | null,
 *   fetchImpl?: typeof fetch,
 *   onUnauthenticated?: () => void,
 *   onClockOffset?: (offsetMs: number) => void,
 * }} [opts]
 */
export function createApiClient(opts = {}) {
  const baseUrl = opts.baseUrl ?? '/api/v1';
  const fetchImpl = opts.fetchImpl ?? globalThis.fetch.bind(globalThis);
  const getBusinessId = opts.getBusinessId ?? (() => null);
  const getCsrfToken = opts.getCsrfToken ?? (() => null);

  async function request(method, path, { body, rawBody, idempotencyKey, version, signal, responseType } = {}) {
    const headers = { Accept: 'application/json' };

    const businessId = getBusinessId();
    if (businessId) headers['X-Business-ID'] = businessId;

    if (body !== undefined) headers['Content-Type'] = 'application/json';
    if (rawBody !== undefined) {
      if (body !== undefined || typeof rawBody !== 'string') throw new Error('Payload CSV harus berupa teks tanpa body JSON.');
      headers['Content-Type'] = 'text/csv; charset=utf-8';
    }
    if (idempotencyKey) headers['Idempotency-Key'] = idempotencyKey;
    if (version !== undefined) headers['If-Match'] = String(version);

    const csrf = getCsrfToken();
    if (csrf && method !== 'GET') headers['X-CSRF-Token'] = csrf;

    let res;
    const sentAt = Date.now();
    try {
      res = await fetchImpl(`${baseUrl}${path}`, {
        method,
        headers,
        credentials: 'include',
        body: rawBody !== undefined ? rawBody : body === undefined ? undefined : JSON.stringify(body),
        signal,
      });
    } catch (cause) {
      if (cause?.name === 'AbortError') throw cause;
      throw new ApiError({
        code: 'NETWORK',
        message: 'Tidak dapat menghubungi server. Periksa koneksi lalu coba lagi.',
      });
    }

    const requestId = res.headers.get('X-Request-ID') || undefined;
    const receivedAt = Date.now();
    const serverTime = Date.parse(res.headers.get('Date') ?? '');
    if (Number.isFinite(serverTime) && receivedAt >= sentAt && receivedAt - sentAt < 10000) {
      opts.onClockOffset?.(serverTime - (sentAt + receivedAt) / 2);
    }

    if (res.status === 204) return undefined;
    if (res.ok && responseType === 'blob') return res.blob();

    let payload = undefined;
    const text = await res.text();
    if (text) {
      try {
        payload = JSON.parse(text);
      } catch {
        payload = undefined;
      }
    }

    if (!res.ok) {
      if (res.status === 401 && path !== '/auth/login') opts.onUnauthenticated?.();
      const errBody = payload?.error ?? {};
      throw new ApiError({
        code: errBody.code ?? `HTTP_${res.status}`,
        message: errBody.message ?? 'Permintaan gagal.',
        details: errBody.details,
        requestId: errBody.request_id ?? requestId,
        status: res.status,
      });
    }

    return payload;
  }

  return {
    get: (path, o) => request('GET', path, o),
    post: (path, body, o) => request('POST', path, { ...o, body }),
    put: (path, body, o) => request('PUT', path, { ...o, body }),
    patch: (path, body, o) => request('PATCH', path, { ...o, body }),
    del: (path, o) => request('DELETE', path, o),
    request,
  };
}

/**
 * Membuat Idempotency-Key baru (UUID v4).
 * Satu percobaan finalisasi = satu key. UI tidak boleh membuat key baru untuk
 * keranjang yang sama sebelum mendapat respons pasti (PRD §13.12).
 */
export function newIdempotencyKey() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
  // Fallback untuk lingkungan tanpa crypto.randomUUID (jarang; hanya dev lama).
  const b = new Uint8Array(16);
  globalThis.crypto.getRandomValues(b);
  b[6] = (b[6] & 0x0f) | 0x40;
  b[8] = (b[8] & 0x3f) | 0x80;
  const h = [...b].map((x) => x.toString(16).padStart(2, '0')).join('');
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}
