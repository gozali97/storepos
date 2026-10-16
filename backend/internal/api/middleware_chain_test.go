package apiserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

type chainVerifier struct {
	calls *[]string
	err   error
}

func (v chainVerifier) Verify(context.Context, string) (SessionInfo, error) {
	*v.calls = append(*v.calls, "auth")
	return SessionInfo{UserID: idkit.New().String(), ActiveBiz: idkit.New().String()}, v.err
}
func (v chainVerifier) Touch(context.Context, string) {}

type chainResolver struct {
	calls *[]string
	tc    tenant.Context
	err   error
}

func (v chainResolver) Resolve(context.Context, string, string) (tenant.Context, error) {
	*v.calls = append(*v.calls, "tenant")
	return v.tc, v.err
}

func TestMiddlewareChainStopsAtFirstRejectedLayer(t *testing.T) {
	cases := []struct {
		name                       string
		csrf                       bool
		authErr, tenantErr         error
		module, permission, branch bool
		want                       int
		calls                      []string
	}{
		{"csrf", false, nil, nil, true, true, true, 403, nil},
		{"auth", true, errs.Unauthenticated(""), nil, true, true, true, 401, []string{"auth"}},
		{"tenant", true, nil, errs.PermissionDenied(""), true, true, true, 403, []string{"auth", "tenant"}},
		{"module", true, nil, nil, false, true, true, 403, []string{"auth", "tenant"}},
		{"permission", true, nil, nil, true, false, true, 403, []string{"auth", "tenant", "permission"}},
		{"branch", true, nil, nil, true, true, false, 403, []string{"auth", "tenant", "permission", "branch"}},
		{"success", true, nil, nil, true, true, true, 204, []string{"auth", "tenant", "permission", "branch", "handler"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var calls []string
			branch := idkit.New()
			tc := tenant.Context{Role: tenant.RoleCashier, BusinessID: idkit.New(), UserID: idkit.New(), MembershipID: idkit.New()}
			if c.module {
				tc.EnabledModules = []string{"restaurant"}
			}
			if c.branch {
				tc.BranchIDs = []idkit.ID{branch}
			}
			endpoint := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, "branch")
				if _, ok := branchOf(w, r, tenant.MustFromContext(r.Context()), branch.String()); !ok {
					return
				}
				calls = append(calls, "handler")
				w.WriteHeader(204)
			})
			check := func(tenant.Context) error {
				calls = append(calls, "permission")
				if !c.permission {
					return errs.PermissionDenied("")
				}
				return nil
			}
			h := CSRFMiddleware(false)(RequireAuth(chainVerifier{&calls, c.authErr})(RequireTenant(chainResolver{&calls, tc, c.tenantErr})(RequireModule("restaurant")(RequirePermission(check)(endpoint)))))
			req := httptest.NewRequest("POST", "/chain", nil)
			req.AddCookie(&http.Cookie{Name: CookieSession, Value: "session"})
			if c.csrf {
				req.AddCookie(&http.Cookie{Name: CookieCSRF, Value: "csrf"})
				req.Header.Set(HeaderCSRF, "csrf")
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.want || !reflect.DeepEqual(calls, c.calls) {
				t.Fatalf("status=%d calls=%v; want status=%d calls=%v", rec.Code, calls, c.want, c.calls)
			}
		})
	}
}
