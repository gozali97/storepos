import { useCallback, useEffect, useState } from 'react';
import { useSession } from '../auth/SessionContext.jsx';

export function useResource(path) {
  const { client, businessId } = useSession();
  const [revision, setRevision] = useState(0);
  const [state, setState] = useState({ data: null, error: null, loading: !!path });
  useEffect(() => {
    const controller = new AbortController();
    setState({ data: null, error: null, loading: !!path, path, businessId });
    if (path) client.get(path, { signal: controller.signal }).then(
      data => { if (!controller.signal.aborted) setState({ data, error: null, loading: false, path, businessId }); },
      error => { if (!controller.signal.aborted) setState({ data: null, error, loading: false, path, businessId }); },
    );
    return () => controller.abort();
  }, [path, businessId, client, revision]);
  const reload = useCallback(() => setRevision(n => n + 1), []);
  const current = state.path === path && state.businessId === businessId;
  return { ...(current ? state : { data: null, error: null, loading: !!path }), reload };
}
