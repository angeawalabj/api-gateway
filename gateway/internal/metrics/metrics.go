package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{Namespace: "gateway", Name: "requests_total", Help: "Total requêtes."},
		[]string{"tenant", "method", "status"},
	)
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gateway", Name: "request_duration_seconds", Help: "Latence end-to-end.",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"tenant", "method"},
	)
	RateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{Namespace: "gateway", Name: "rate_limit_hits_total", Help: "Requêtes bloquées."},
		[]string{"tenant", "window"},
	)
	ActiveConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Namespace: "gateway", Name: "active_connections", Help: "Connexions actives."},
		[]string{"tenant"},
	)
	UpstreamErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{Namespace: "gateway", Name: "upstream_errors_total", Help: "Erreurs upstream."},
		[]string{"tenant", "status"},
	)
)

func Handler() http.Handler { return promhttp.Handler() }

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start  := time.Now()
		rw     := &sw{ResponseWriter: w, status: 200}
		tenant := tenantFromCtx(r)

		ActiveConnections.WithLabelValues(tenant).Inc()
		defer ActiveConnections.WithLabelValues(tenant).Dec()

		next.ServeHTTP(rw, r)

		status := strconv.Itoa(rw.status)
		RequestsTotal.WithLabelValues(tenant, r.Method, status).Inc()
		RequestDuration.WithLabelValues(tenant, r.Method).Observe(time.Since(start).Seconds())
		if rw.status >= 500 {
			UpstreamErrors.WithLabelValues(tenant, status).Inc()
		}
	})
}

type sw struct {
	http.ResponseWriter
	status  int
	written bool
}

func (s *sw) WriteHeader(code int) {
	if !s.written {
		s.status  = code
		s.written = true
		s.ResponseWriter.WriteHeader(code)
	}
}

func tenantFromCtx(r *http.Request) string {
	if v := r.Context().Value("tenant_id"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "unknown"
}
