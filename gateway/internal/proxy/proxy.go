package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/ton-user/api-gateway/internal/middleware"
)

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream := middleware.UpstreamFrom(r.Context())
		tenantID := middleware.TenantIDFrom(r.Context())

		if upstream == "" {
			slog.Error("no upstream in context", "path", r.URL.Path)
			http.Error(w, `{"error":"no upstream configured"}`, http.StatusBadGateway)
			return
		}

		target, err := url.Parse(upstream)
		if err != nil {
			slog.Error("invalid upstream URL", "upstream", upstream, "err", err)
			http.Error(w, `{"error":"invalid upstream"}`, http.StatusBadGateway)
			return
		}

		newReverseProxy(target, tenantID).ServeHTTP(w, r)
	})
}

func newReverseProxy(target *url.URL, tenantID string) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.Transport = &http.Transport{
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}

	orig := proxy.Director
	proxy.Director = func(req *http.Request) {
		orig(req)
		req.Header.Set("X-Forwarded-For",   req.RemoteAddr)
		req.Header.Set("X-Tenant-ID",       tenantID)
		req.Header.Set("X-Gateway-Version", "1.0")
		req.Header.Del("X-API-Key")
		req.Header.Del("Authorization")
		req.URL.Host   = target.Host
		req.URL.Scheme = target.Scheme
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set("X-Served-By", "api-gateway")
		resp.Header.Set("X-Tenant-ID", tenantID)
		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("upstream error", "tenant", tenantID, "path", r.URL.Path, "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":"upstream unavailable","status":502}`))
	}

	return proxy
}
