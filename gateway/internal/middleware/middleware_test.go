package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ton-user/api-gateway/internal/middleware"
)

// ─── Mocks ────────────────────────────────────────────────────────────────────

type mockTenant struct{ id, name, upstream string; ps, pm, pd int }

func (t *mockTenant) GetID() string                  { return t.id }
func (t *mockTenant) GetName() string                { return t.name }
func (t *mockTenant) GetUpstream() string            { return t.upstream }
func (t *mockTenant) GetRateLimits() (int, int, int) { return t.ps, t.pm, t.pd }

type mockRepo struct{ tenants map[string]*mockTenant }

func (r *mockRepo) GetByAPIKey(_ context.Context, key string) (middleware.TenantInfo, error) {
	t, ok := r.tenants[key]
	if !ok {
		return nil, middleware.ErrTenantNotFound
	}
	return t, nil
}

func newRepo() *mockRepo {
	return &mockRepo{tenants: map[string]*mockTenant{
		"key-a": {id: "t-a", name: "Tenant A", upstream: "http://svc-a:8081", ps: 10, pm: 100, pd: 10000},
		"key-b": {id: "t-b", name: "Tenant B", upstream: "http://svc-b:8082", ps: 5,  pm: 50,  pd: 5000},
	}}
}

type mockResult struct{ allowed bool; limit, remaining int; resetAt time.Time; window string }

func (r *mockResult) IsAllowed() bool       { return r.allowed }
func (r *mockResult) GetLimit() int         { return r.limit }
func (r *mockResult) GetRemaining() int     { return r.remaining }
func (r *mockResult) GetResetAt() time.Time { return r.resetAt }
func (r *mockResult) GetWindow() string     { return r.window }

type mockLimiter struct{ result *mockResult }

func (l *mockLimiter) Check(_ context.Context, _ string, ps, _, _ int) middleware.RateLimitResult {
	if l.result != nil {
		return l.result
	}
	return &mockResult{allowed: true, limit: ps, remaining: ps - 1, resetAt: time.Now().Add(time.Second), window: "per_second"}
}

func ok200(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

// ─── TenantResolve tests ──────────────────────────────────────────────────────

func TestTenantResolve_MissingKey(t *testing.T) {
	h := middleware.TenantResolve(newRepo())(http.HandlerFunc(ok200))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/", nil))
	if rw.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rw.Code)
	}
}

func TestTenantResolve_InvalidKey(t *testing.T) {
	h := middleware.TenantResolve(newRepo())(http.HandlerFunc(ok200))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "bad-key")
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rw.Code)
	}
}

func TestTenantResolve_ValidKey(t *testing.T) {
	var gotID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = middleware.TenantIDFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	h := middleware.TenantResolve(newRepo())(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "key-a")
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rw.Code)
	}
	if gotID != "t-a" {
		t.Errorf("expected tenant_id=t-a, got %q", gotID)
	}
}

func TestTenantResolve_InjectsUpstream(t *testing.T) {
	var gotUpstream string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUpstream = middleware.UpstreamFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	h := middleware.TenantResolve(newRepo())(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "key-b")
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if gotUpstream != "http://svc-b:8082" {
		t.Errorf("expected http://svc-b:8082, got %q", gotUpstream)
	}
}

// ─── RateLimit tests ──────────────────────────────────────────────────────────

func TestRateLimit_AllowedSetsHeaders(t *testing.T) {
	h := middleware.RateLimit(&mockLimiter{}, newRepo())(http.HandlerFunc(ok200))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "key-a")
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rw.Code)
	}
	for _, hdr := range []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		if rw.Header().Get(hdr) == "" {
			t.Errorf("missing header %s", hdr)
		}
	}
}

func TestRateLimit_Blocked429(t *testing.T) {
	lim := &mockLimiter{result: &mockResult{
		allowed: false, limit: 10, remaining: 0,
		resetAt: time.Now().Add(time.Second), window: "per_second",
	}}
	h := middleware.RateLimit(lim, newRepo())(http.HandlerFunc(ok200))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "key-a")
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rw.Code)
	}
	if rw.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header on 429")
	}
}

// ─── CORS tests ───────────────────────────────────────────────────────────────

func TestCORS_SetsHeader(t *testing.T) {
	h := middleware.CORS([]string{"http://localhost:3000"})(http.HandlerFunc(ok200))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/", nil))
	if rw.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("missing CORS header")
	}
}

func TestCORS_PreflightReturns204(t *testing.T) {
	h := middleware.CORS([]string{"*"})(http.HandlerFunc(ok200))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, httptest.NewRequest(http.MethodOptions, "/", nil))
	if rw.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rw.Code)
	}
}

// ─── Recovery tests ───────────────────────────────────────────────────────────

func TestRecovery_CatchesPanic(t *testing.T) {
	h := middleware.Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/", nil))
	if rw.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rw.Code)
	}
}
