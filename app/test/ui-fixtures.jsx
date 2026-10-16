export function useSession() { return globalThis.__storeposTest.session; }
export function useWorkspace() { return globalThis.__storeposTest.workspace; }
export function useResource(path) { return globalThis.__storeposTest.resource(path); }
