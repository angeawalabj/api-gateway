package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ton-user/api-gateway/internal/config"
	"github.com/ton-user/api-gateway/internal/db"
	"github.com/ton-user/api-gateway/internal/metrics"
	"github.com/ton-user/api-gateway/internal/middleware"
	"github.com/ton-user/api-gateway/internal/ratelimit"
	"github.com/ton-user/api-gateway/internal/router"
	"github.com/ton-user/api-gateway/internal/tenant"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	setupLogger(cfg.LogLevel)
	slog.Info("starting api-gateway", "addr", cfg.ListenAddr, "dev_mode", cfg.DevMode)

	// ── PostgreSQL ────────────────────────────────────────────────────────────
	database, err := db.Connect(cfg.PostgresDSN)
	if err != nil {
		slog.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		slog.Error("db migrate failed", "err", err)
		os.Exit(1)
	}

	// ── Redis adapter ─────────────────────────────────────────────────────────
	redisClient := newRedisAdapter(cfg)
	limiter     := ratelimit.New(redisClient)

	// ── Tenant repository ─────────────────────────────────────────────────────
	tenantRepo := tenant.NewRepository(database)

	// ── Router ────────────────────────────────────────────────────────────────
	handler := router.New(router.Deps{
		TenantRepo:   &tenantRepoAdapter{repo: tenantRepo},
		RateLimiter:  &rateLimiterAdapter{l: limiter},
		DevMode:      cfg.DevMode,
		AllowOrigins: []string{"http://localhost:3000", "http://localhost:5173"},
	})

	// ── Serveur HTTP principal ────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// ── Métriques Prometheus ──────────────────────────────────────────────────
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metrics.Handler())
	metricsSrv := &http.Server{Addr: cfg.MetricsAddr, Handler: metricsMux}

	go func() {
		slog.Info("metrics server", "addr", cfg.MetricsAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics server error", "err", err)
		}
	}()

	go func() {
		slog.Info("gateway listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("gateway error", "err", err)
			os.Exit(1)
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_ = metricsSrv.Shutdown(ctx)
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "err", err)
		os.Exit(1)
	}
	slog.Info("stopped")
}

func setupLogger(level string) {
	var l slog.Level
	switch level {
	case "debug":  l = slog.LevelDebug
	case "warn":   l = slog.LevelWarn
	case "error":  l = slog.LevelError
	default:       l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l})))
}

// ─── Adapters ─────────────────────────────────────────────────────────────────

type tenantRepoAdapter struct{ repo *tenant.Repository }

func (a *tenantRepoAdapter) GetByAPIKey(ctx context.Context, apiKey string) (middleware.TenantInfo, error) {
	return a.repo.GetByAPIKeyHash(ctx, apiKey)
}

type rateLimiterAdapter struct{ l *ratelimit.Limiter }

func (a *rateLimiterAdapter) Check(ctx context.Context, id string, ps, pm, pd int) middleware.RateLimitResult {
	return &rlResult{r: a.l.Check(ctx, id, ps, pm, pd)}
}

type rlResult struct{ r ratelimit.Result }

func (r *rlResult) IsAllowed() bool          { return r.r.Allowed }
func (r *rlResult) GetLimit() int            { return r.r.Limit }
func (r *rlResult) GetRemaining() int        { return r.r.Remaining }
func (r *rlResult) GetResetAt() time.Time    { return r.r.ResetAt }
func (r *rlResult) GetWindow() string        { return r.r.Window }

// redisAdapter est un no-op — remplacé par go-redis en vrai déploiement
type redisAdapter struct{ addr, password string; dbNum int }

func newRedisAdapter(cfg *config.Config) ratelimit.RedisClient {
	return &redisAdapter{addr: cfg.RedisAddr, password: cfg.RedisPassword, dbNum: cfg.RedisDB}
}

func (r *redisAdapter) Eval(_ context.Context, _ string, _ []string, _ ...any) (any, error) {
	// Production : go-redis Client.Eval()
	// Dev sans Redis : retourne toujours "autorisé"
	return []any{int64(1), int64(0), int64(0)}, nil
}

func (r *redisAdapter) Get(_ context.Context, _ string) (string, error) {
	return "0", nil
}
