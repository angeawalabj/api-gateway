package router

import (
	"encoding/json"
	"net/http"

	"github.com/ton-user/api-gateway/internal/metrics"
	"github.com/ton-user/api-gateway/internal/middleware"
	"github.com/ton-user/api-gateway/internal/proxy"
)

type Deps struct {
	TenantRepo   middleware.TenantRepo
	RateLimiter  middleware.RateLimiter
	DevMode      bool
	AllowOrigins []string
}

func New(deps Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": "1.0.0"})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	// Routes proxy
	proxyHandler := chain(deps, proxy.Handler())
	mux.Handle("/", proxyHandler)

	// Admin placeholder (étendu par les handlers CRUD)
	mux.HandleFunc("GET /admin/tenants", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"tenants": []any{}, "total": 0})
	})

	return mux
}

func chain(deps Deps, h http.Handler) http.Handler {
	// Ordre d'exécution : Recovery → Logger → CORS → Metrics → TenantResolve → RateLimit → Proxy
	h = middleware.RateLimit(deps.RateLimiter, deps.TenantRepo)(h)
	h = middleware.TenantResolve(deps.TenantRepo)(h)
	h = metrics.Middleware(h)
	h = middleware.CORS(deps.AllowOrigins)(h)
	h = middleware.Logger(h)
	h = middleware.Recovery(h)
	return h
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
