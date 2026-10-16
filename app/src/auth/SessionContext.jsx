import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { createApiClient } from '../api/client.js';

const SessionContext = createContext(null);

/**
 * Mengelola sesi pengguna: status login, konteks bisnis aktif, dan klien API.
 * Cookie sesi bersifat HttpOnly (§16.5), jadi frontend tidak menyimpan token —
 * hanya mengingat pilihan bisnis aktif di localStorage.
 */
export function SessionProvider({ children, apiClient }) {
  const [status, setStatus] = useState('memuat'); // memuat | tamu | masuk | gagal
  const [user, setUser] = useState(null);
  const [memberships, setMemberships] = useState([]);
  const [businessId, setBusinessId] = useState(() => localStorage.getItem('storepos.businessId'));
  const [error, setError] = useState(null);
  const [clockOffsetMs, setClockOffsetMs] = useState(null);
  const updateDisplayName = useCallback((name) => {
    setUser((current) => current ? { ...current, name } : current);
  }, []);

  const client = useMemo(
    () =>
      apiClient ??
      createApiClient({
        getBusinessId: () => localStorage.getItem('storepos.businessId'),
        getCsrfToken: () => readCookie('storepos_csrf'),
        onClockOffset: setClockOffsetMs,
        onUnauthenticated: () => {
          localStorage.removeItem('storepos.businessId');
          setBusinessId(null);
          setUser(null);
          setMemberships([]);
          setStatus('tamu');
          setError(null);
        },
      }),
    [apiClient],
  );

  const muat = useCallback(async () => {
    setStatus('memuat');
    try {
      const [me, profile] = await Promise.all([
        client.get('/me'),
        client.get('/me/profile').catch(() => null),
      ]);
      setUser(profile?.display_name ? { ...me.user, name: profile.display_name } : me.user);
      setMemberships(me.memberships ?? []);
      const aktif = me.memberships?.find((m) => m.business_id === localStorage.getItem('storepos.businessId'));
      const dipilih = aktif ?? me.memberships?.[0];
      if (dipilih) {
        localStorage.setItem('storepos.businessId', dipilih.business_id);
        setBusinessId(dipilih.business_id);
      } else {
        localStorage.removeItem('storepos.businessId');
        setBusinessId(null);
      }
      setStatus('masuk');
      setError(null);
    } catch (err) {
      if (err.code === 'UNAUTHENTICATED' || err.status === 401) {
        setStatus('tamu');
        setUser(null);
        setMemberships([]);
        localStorage.removeItem('storepos.businessId');
        setBusinessId(null);
        setError(null);
      } else {
        setStatus('gagal');
        setError(err);
      }
    }
  }, [client]);

  useEffect(() => {
    muat();
  }, [muat]);

  const masuk = useCallback(
    async (email, kataSandi) => {
      const res = await client.post('/auth/login', { email, password: kataSandi });
      const profile = await client.get('/me/profile').catch(() => null);
      setUser(profile?.display_name ? { ...res.user, name: profile.display_name } : res.user);
      setMemberships(res.memberships ?? []);
      const pertama = res.memberships?.[0];
      if (pertama) {
        localStorage.setItem('storepos.businessId', pertama.business_id);
        setBusinessId(pertama.business_id);
      } else {
        localStorage.removeItem('storepos.businessId');
        setBusinessId(null);
      }
      setStatus('masuk');
      setError(null);
      return res;
    },
    [client],
  );

  const keluar = useCallback(async () => {
    await client.post('/auth/logout', {});
    localStorage.removeItem('storepos.businessId');
    setBusinessId(null);
    setUser(null);
    setMemberships([]);
    setStatus('tamu');
  }, [client]);

  const gantiBisnis = useCallback((id) => {
    if (!memberships.some(m => m.business_id === id)) return;
    localStorage.setItem('storepos.businessId', id);
    setBusinessId(id);
  }, [memberships]);

  const value = useMemo(
    () => ({
      status,
      user,
      memberships,
      businessId,
      business: memberships.find((m) => m.business_id === businessId) ?? null,
      error,
      clockOffsetMs,
      client,
      masuk,
      keluar,
      gantiBisnis,
      muatUlang: muat,
      updateDisplayName,
    }),
    [status, user, memberships, businessId, error, clockOffsetMs, client, masuk, keluar, gantiBisnis, muat, updateDisplayName],
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession() {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error('useSession harus dipakai di dalam <SessionProvider>');
  return ctx;
}

function readCookie(nama) {
  const m = document.cookie.match(new RegExp(`(?:^|; )${nama}=([^;]*)`));
  return m ? decodeURIComponent(m[1]) : null;
}
