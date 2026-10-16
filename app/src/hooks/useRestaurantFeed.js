import { useCallback, useEffect, useRef, useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';

// SSE hanya invalidasi; API sesi+header bisnis tetap sumber data. Polling menjaga fallback tanpa stream.
export function useRestaurantFeed(path, branchId, collection) {
  const { client } = useSession();
  const run = useRef(() => {});
  const [state, setState] = useState({ data: null, error: null, loading: !!path, path });
  const reload = useCallback(() => run.current(), []);
  useEffect(() => {
    let active = true;
    let inFlight = null;
    let refreshAgain = false;
    let source;
    let timer;
    const controller = new AbortController();
    setState({ data: null, error: null, loading: !!path, path, stream: 'connecting' });
    if (!path) { run.current = () => {}; return () => controller.abort(); }
    function streamStatus(stream) {
      if (active) setState(current => ({ ...current, stream }));
    }
    async function load() {
      if (!active) return;
      if (inFlight) { refreshAgain = true; return inFlight; }
      inFlight = (async () => {
        try {
          let url = path;
          let result;
          const items = [];
          const cursors = new Set();
          for (let page = 0; ; page += 1) {
            // shortcut: 10.000 item per layar; tambah virtualisasi/server filter bila operasi melewati batas ini.
            if (page >= 100) throw new Error('Data melebihi 10.000 item. Persempit cakupan sebelum melanjutkan.');
            const data = await client.get(url, { signal: controller.signal });
            result = data;
            if (!collection) break;
            items.push(...(data[collection] ?? []));
            if (!data.next_cursor) break;
            if (cursors.has(data.next_cursor)) throw new Error('Cursor berulang dari server; muat ulang data.');
            cursors.add(data.next_cursor);
            const next = new URL(path, window.location.origin);
            next.searchParams.set('after', data.next_cursor);
            url = next.pathname + next.search;
          }
          if (collection) result = { ...result, [collection]: items, next_cursor: '' };
          if (active) setState(current => ({ ...current, data: result, error: null, loading: false, receivedAt: Date.now() }));
        } catch (error) {
          if (active && error.name !== 'AbortError') setState(current => ({ ...current, error, loading: false }));
        }
      })();
      try { await inFlight; }
      finally {
        inFlight = null;
        if (active && refreshAgain) { refreshAgain = false; void load(); }
      }
    }
    run.current = load;
    function reconnect() {
      source?.close();
      if (!navigator.onLine) { streamStatus('offline'); return; }
      void load();
      if (!branchId || typeof EventSource === 'undefined') { streamStatus('polling'); return; }
      streamStatus('connecting');
      source = new EventSource(`/api/v1/restaurant/events?branch_id=${encodeURIComponent(branchId)}`, { withCredentials: true });
      source.onopen = () => streamStatus('connected');
      source.onerror = () => streamStatus('polling');
      source.addEventListener('refresh', load);
    }
    function offline() { source?.close(); streamStatus('offline'); }
    function visible() { if (document.visibilityState === 'visible') void load(); }
    reconnect();
    timer = setInterval(() => { if (navigator.onLine) void load(); }, 5000);
    window.addEventListener('online', reconnect);
    window.addEventListener('offline', offline);
    document.addEventListener('visibilitychange', visible);
    return () => {
      active = false;
      controller.abort();
      source?.close();
      clearInterval(timer);
      run.current = () => {};
      window.removeEventListener('online', reconnect);
      window.removeEventListener('offline', offline);
      document.removeEventListener('visibilitychange', visible);
    };
  }, [path, branchId, collection, client]);
  if (state.path !== path) return { data: null, error: null, loading: !!path, reload, stream: 'connecting' };
  return { ...state, reload };
}
