import { useRef, useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';
import { loadPendingMutation, performPendingMutation } from '../pendingMutation.js';

// Pemanggil wajib remount saat scope (pengguna/bisnis/cabang/order) berubah.
export function usePendingMutation(storageKey) {
  const { client } = useSession();
  const lock = useRef(false);
  const [state, setState] = useState(() => readState(storageKey));
  const [sending, setSending] = useState(false);
  async function send(operation) {
    if (lock.current) throw new Error('Aksi masih diproses.');
    if (state.error) throw new Error(state.error);
    lock.current = true;
    setSending(true);
    try {
      const op = state.pending ?? operation;
      setState({ pending: op, error: '' });
      return await performPendingMutation(client, sessionStorage, storageKey, op);
    } finally {
      setState(readState(storageKey));
      lock.current = false;
      setSending(false);
    }
  }
  return { ...state, sending, blocked: !!state.pending || !!state.error || sending, send };
}

function readState(key) {
  try { return { pending: loadPendingMutation(sessionStorage, key), error: '' }; }
  catch { return { pending: null, error: 'Data pemulihan tidak dapat dibaca. Jangan ulangi aksi; periksa status di server terlebih dahulu.' }; }
}
