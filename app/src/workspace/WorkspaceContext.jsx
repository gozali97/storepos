import { createContext, useContext, useEffect, useState } from "react";
import { useSession } from "../auth/SessionContext.jsx";
import { useResource } from "../hooks/useResource.js";
const WorkspaceContext = createContext(null);
export function WorkspaceProvider({
  children
}) {
  const {
    businessId
  } = useSession();
  const resource = useResource(businessId ? "/workspace" : null);
  const storageKey = `storepos.workspace.${businessId}`;
  const [selection, setSelection] = useState(() => {
    try {
      return JSON.parse(localStorage.getItem(storageKey)) || {};
    } catch {
      return {};
    }
  });
  const branches = resource.data?.branches ?? [];
  const branch = branches.find(b => b.id === selection.branchId) ?? branches[0];
  const registers = (resource.data?.registers ?? []).filter(r => r.branch_id === branch?.id);
  const register = registers.find(r => r.id === selection.registerId) ?? registers[0];
  useEffect(() => {
    if (resource.data) localStorage.setItem(storageKey, JSON.stringify({
      branchId: branch?.id,
      registerId: register?.id
    }));
  }, [storageKey, resource.data, branch?.id, register?.id]);
  return <WorkspaceContext.Provider value={{
    ...resource,
    branch,
    register,
    branches,
    registers,
    selectBranch: branchId => setSelection({
      branchId
    }),
    selectRegister: registerId => setSelection({
      branchId: branch?.id,
      registerId
    })
  }}>{children}</WorkspaceContext.Provider>;
}
export function useWorkspace() {
  return useContext(WorkspaceContext);
}
