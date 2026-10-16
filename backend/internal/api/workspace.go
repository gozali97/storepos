package apiserver

import (
	"net/http"

	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

type workspaceBranch struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

type workspaceRegister struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	BranchID string `json:"branch_id"`
}

func (s *Server) workspace(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT b.id, b.name, b.timezone,
 COALESCE(r.id::text,''), COALESCE(r.name,'') FROM branches b
 LEFT JOIN registers r ON r.business_id=b.business_id AND r.branch_id=b.id AND r.status='active'
 WHERE b.business_id=$1 AND b.status='active' ORDER BY b.name, r.name`, tc.BusinessID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	branches := []workspaceBranch{}
	registers := []workspaceRegister{}
	seen := map[string]bool{}
	for rows.Next() {
		var id idkit.ID
		var name, timezone, registerID, registerName string
		if err := rows.Scan(&id, &name, &timezone, &registerID, &registerName); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if !tc.HasBranch(id) {
			continue
		}
		bid := id.String()
		if !seen[bid] {
			branches = append(branches, workspaceBranch{bid, name, timezone})
			seen[bid] = true
		}
		if registerID != "" {
			registers = append(registers, workspaceRegister{registerID, registerName, bid})
		}
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"branches": branches, "registers": registers, "enabled_modules": tc.EnabledModules})
}
