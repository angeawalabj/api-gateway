package ratelimit_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ton-user/api-gateway/internal/ratelimit"
)

type mockRedis struct {
	mu   sync.Mutex
	data map[string]int64
}

func newMockRedis() *mockRedis {
	return &mockRedis{data: make(map[string]int64)}
}

func (m *mockRedis) Eval(_ context.Context, _ string, keys []string, args ...any) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	curKey  := keys[0]
	prevKey := keys[1]
	limit   := int64(toInt(args[0]))
	window  := int64(toInt(args[1]))
	elapsed := int64(toInt(args[2]))

	m.data[curKey]++
	current := m.data[curKey]

	prev   := m.data[prevKey]
	weight := float64(elapsed) / float64(window*1000)
	est    := float64(prev)*(1-weight) + float64(current)

	isLimited := int64(0)
	if est > float64(limit) {
		isLimited = 1
	}
	return []any{current, isLimited, prev}, nil
}

func (m *mockRedis) Get(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[key]
	if !ok {
		return "", fmt.Errorf("key not found")
	}
	return fmt.Sprintf("%d", v), nil
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:     return n
	case int64:   return int(n)
	case float64: return int(n)
	}
	return 0
}

func TestLimiter_AllowedUnderLimit(t *testing.T) {
	lim := ratelimit.New(newMockRedis())
	ctx := context.Background()
	for i := range 5 {
		r := lim.Check(ctx, "tenant-a", 10, 100, 10000)
		if !r.Allowed {
			t.Errorf("req %d: expected allowed", i+1)
		}
	}
}

func TestLimiter_BlockedOverLimit(t *testing.T) {
	lim := ratelimit.New(newMockRedis())
	ctx := context.Background()
	for range 3 {
		lim.Check(ctx, "tenant-b", 3, 300, 30000)
	}
	r := lim.Check(ctx, "tenant-b", 3, 300, 30000)
	if r.Allowed {
		t.Error("4th request should be blocked")
	}
	if r.Window != "per_second" {
		t.Errorf("expected per_second, got %s", r.Window)
	}
}

func TestLimiter_DifferentTenantsIndependent(t *testing.T) {
	lim := ratelimit.New(newMockRedis())
	ctx := context.Background()
	for range 5 {
		lim.Check(ctx, "tenant-x", 5, 50, 5000)
	}
	if lim.Check(ctx, "tenant-x", 5, 50, 5000).Allowed {
		t.Error("tenant-x should be limited")
	}
	if !lim.Check(ctx, "tenant-y", 5, 50, 5000).Allowed {
		t.Error("tenant-y should not be affected")
	}
}

func TestLimiter_FailOpenOnRedisError(t *testing.T) {
	lim := ratelimit.New(&errorRedis{})
	r   := lim.Check(context.Background(), "tenant-c", 10, 100, 10000)
	if !r.Allowed {
		t.Error("should fail-open when Redis unavailable")
	}
}

func TestLimiter_DisabledWindowPerSecond(t *testing.T) {
	lim := ratelimit.New(newMockRedis())
	ctx := context.Background()
	// limit=0 désactive per_second, per_minute limit=5
	for range 5 {
		lim.Check(ctx, "tenant-d", 0, 5, 1000)
	}
	r := lim.Check(ctx, "tenant-d", 0, 5, 1000)
	if r.Allowed {
		t.Error("per_minute limit should have been enforced")
	}
	if r.Window != "per_minute" {
		t.Errorf("expected per_minute, got %s", r.Window)
	}
}

func TestLimiter_HeadersPopulated(t *testing.T) {
	lim := ratelimit.New(newMockRedis())
	r   := lim.Check(context.Background(), "tenant-e", 100, 1000, 100000)
	if r.Limit <= 0 {
		t.Errorf("Limit should be positive, got %d", r.Limit)
	}
	if r.Remaining < 0 {
		t.Errorf("Remaining should be >= 0, got %d", r.Remaining)
	}
	if r.ResetAt.IsZero() || r.ResetAt.Before(time.Now()) {
		t.Error("ResetAt should be in the future")
	}
}

type errorRedis struct{}
func (e *errorRedis) Eval(_ context.Context, _ string, _ []string, _ ...any) (any, error) {
	return nil, fmt.Errorf("redis: connection refused")
}
func (e *errorRedis) Get(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("redis: connection refused")
}
