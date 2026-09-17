package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Result struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time
	Window    string
}

type RedisClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
	Get(ctx context.Context, key string) (string, error)
}

type Limiter struct {
	redis RedisClient
}

func New(redis RedisClient) *Limiter {
	return &Limiter{redis: redis}
}

const luaScript = `
local current_key = KEYS[1]
local prev_key    = KEYS[2]
local limit       = tonumber(ARGV[1])
local window_secs = tonumber(ARGV[2])
local elapsed_ms  = tonumber(ARGV[3])

local current = redis.call("INCR", current_key)
if current == 1 then
  redis.call("PEXPIRE", current_key, window_secs * 2000)
end

local prev     = tonumber(redis.call("GET", prev_key) or "0")
local weight   = elapsed_ms / (window_secs * 1000)
local estimated = prev * (1 - weight) + current

local is_limited = 0
if estimated > limit then is_limited = 1 end

return {current, is_limited, prev}
`

func (l *Limiter) Check(
	ctx context.Context,
	tenantID string,
	limitPS, limitPM, limitPD int,
) Result {
	now := time.Now()

	windows := []struct {
		name    string
		secs    int
		limit   int
		tsFunc  func(time.Time) int64
	}{
		{"per_second", 1,     limitPS, func(t time.Time) int64 { return t.Unix() }},
		{"per_minute", 60,    limitPM, func(t time.Time) int64 { return t.Unix() / 60 }},
		{"per_day",    86400, limitPD, func(t time.Time) int64 { return t.Unix() / 86400 }},
	}

	for _, w := range windows {
		if w.limit <= 0 {
			continue
		}

		ts      := w.tsFunc(now)
		prevTS  := ts - 1
		elapsed := now.UnixMilli() % (int64(w.secs) * 1000)

		curKey  := fmt.Sprintf("rl:%s:%s:%d", tenantID, w.name, ts)
		prevKey := fmt.Sprintf("rl:%s:%s:%d", tenantID, w.name, prevTS)

		raw, err := l.redis.Eval(ctx, luaScript,
			[]string{curKey, prevKey},
			w.limit, w.secs, elapsed,
		)
		if err != nil {
			slog.Warn("ratelimit redis error, fail-open",
				"tenant", tenantID, "window", w.name, "err", err)
			continue
		}

		vals, ok := raw.([]any)
		if !ok || len(vals) < 2 {
			continue
		}

		current   := toInt(vals[0])
		isLimited := toInt(vals[1]) == 1
		resetAt   := now.Truncate(time.Duration(w.secs) * time.Second).
			Add(time.Duration(w.secs) * time.Second)

		if isLimited {
			return Result{
				Allowed:   false,
				Limit:     w.limit,
				Remaining: 0,
				ResetAt:   resetAt,
				Window:    w.name,
			}
		}

		// Retourne info de la première fenêtre non bloquante
		remaining := max(0, w.limit-current)
		if w.name == "per_second" {
			return Result{
				Allowed:   true,
				Limit:     w.limit,
				Remaining: remaining,
				ResetAt:   resetAt,
				Window:    w.name,
			}
		}
	}

	return Result{
		Allowed:   true,
		Limit:     limitPS,
		Remaining: limitPS,
		ResetAt:   now.Add(time.Second),
		Window:    "per_second",
	}
}

func toInt(v any) int {
	switch n := v.(type) {
	case int64:   return int(n)
	case int:     return n
	case float64: return int(n)
	}
	return 0
}

func max(a, b int) int {
	if a > b { return a }
	return b
}
