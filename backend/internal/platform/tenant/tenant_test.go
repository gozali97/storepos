package tenant

import (
	"context"
	"testing"

	"github.com/storepos/storepos/internal/platform/idkit"
)

func TestFromContextMissing(t *testing.T) {
	if _, err := FromContext(context.Background()); err == nil {
		t.Fatal("expected error on missing context")
	}
}

func TestFromContextRejectsZeroBusiness(t *testing.T) {
	ctx := WithContext(context.Background(), Context{Role: RoleOwner})
	if _, err := FromContext(ctx); err == nil {
		t.Fatal("expected error for zero business_id")
	}
}

func TestHasBranchOwner(t *testing.T) {
	tc := Context{Role: RoleOwner}
	if !tc.HasBranch(idkit.NewV7()) {
		t.Fatal("owner should have all branches")
	}
}

func TestHasBranchAssigned(t *testing.T) {
	b1 := idkit.NewV7()
	b2 := idkit.NewV7()
	tc := Context{Role: RoleCashier, BranchIDs: []idkit.ID{b1}}
	if !tc.HasBranch(b1) {
		t.Fatal("branch b1 should be assigned")
	}
	if tc.HasBranch(b2) {
		t.Fatal("branch b2 should not be assigned")
	}
}
