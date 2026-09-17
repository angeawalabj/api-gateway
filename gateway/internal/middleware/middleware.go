package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

var ErrTenantNotFound = errors.New("tenant not found")

type contextKey string

const (
	KeyTenantID   contextKey = "tenant_id"
	KeyTenantName contextKey = "tenant_name"
	KeyUpstream   contextKey = "upstream"
)

func TenantIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(KeyTenantID).(string)
	return v
}

func UpstreamFrom(ctx context.Context) string {
	v, _ := ctx.Value(KeyUpstream).(string)
	return v
}

// ─── Interfaces ───────────────────────────────────────────────────────────────

type TenantInfo interface {
	GetID() string
	GetName() string
	GetUpstream() string
	GetRateLimits() (int, int, int)
}

type TenantRepo interface {
	GetByAPIKey(ctx context.Context, apiKey string) (TenantInfo, error)
}

type RateLimitResult interface {
	IsAllowed() bool
	GetLimit() int
	GetRemaining() int
	GetResetAt() time.Time
	GetWindow() string
}

type RateLimiter interface {
	Check(ctx context.Context, tenantID string, ps, pm, pd int) RateLimitResult
}

// ─── Logger ───────────────────────────────────────────────────────────────────

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw    := &responseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		slog.Info("request",
			"method",   r.Method,
			"path",     r.URL.Path,
			"status",   rw.status,
			"duration", time.Since(start).Milliseconds(),
			"tenant",   TenantIDFrom(r.Context()),
			"remote",   r.RemoteAddr,
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// ─── TenantResolve ────────────────────────────────────────────────────────────

func TenantResolve(repo TenantRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				writeError(w, http.StatusUnauthorized, "missing X-API-Key header")
				return
			}
			tenant, err := repo.GetByAPIKey(r.Context(), apiKey)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid API key")
				return
			}
			ctx := context.WithValue(r.Context(), KeyTenantID,   tenant.GetID())
			ctx  = context.WithValue(ctx,          KeyTenantName, tenant.GetName())
			ctx  = context.WithValue(ctx,          KeyUpstream,   tenant.GetUpstream())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ─── RateLimit ────────────────────────────────────────────────────────────────

func RateLimit(limiter RateLimiter, repo TenantRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := TenantIDFrom(r.Context())
			if tenantID == "" {
				next.ServeHTTP(w, r)
				return
			}
			apiKey := r.Header.Get("X-API-Key")
			tenant, err := repo.GetByAPIKey(r.Context(), apiKey)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			ps, pm, pd := tenant.GetRateLimits()
			result := limiter.Check(r.Context(), tenantID, ps, pm, pd)

			w.Header().Set("X-RateLimit-Limit",     fmt.Sprintf("%d", result.GetLimit()))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.GetRemaining()))
			w.Header().Set("X-RateLimit-Reset",     fmt.Sprintf("%d", result.GetResetAt().Unix()))
			w.Header().Set("X-RateLimit-Window",    result.GetWindow())

			if !result.IsAllowed() {
				w.Header().Set("Retry-After", "1")
				writeError(w, http.StatusTooManyRequests,
					fmt.Sprintf("rate limit exceeded (%s)", result.GetWindow()))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ─── JWTAuth ──────────────────────────────────────────────────────────────────

func JWTAuth(_ *rsa.PublicKey, _ string, devMode bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if devMode {
				next.ServeHTTP(w, r)
				return
			}
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "missing Bearer token")
				return
			}
			// TODO: implement RS256 verification via golang-jwt/jwt
			next.ServeHTTP(w, r)
		})
	}
}

// ─── CORS ─────────────────────────────────────────────────────────────────────

func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := strings.Join(allowedOrigins, ", ")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin",  allowed)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key, Authorization")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ─── Recovery ────────────────────────────────────────────────────────────────

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "err", rec, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ─── Helper ──────────────────────────────────────────────────────────────────

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"error": msg, "status": code})
}
