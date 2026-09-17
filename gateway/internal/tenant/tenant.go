package tenant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrNotFound = errors.New("tenant not found")

type Tenant struct {
	ID          string
	Name        string
	APIKeyHash  string
	Upstream    string
	RateLimitPS int
	RateLimitPM int
	RateLimitPD int
	Enabled     bool
	CreatedAt   time.Time
}

func (t *Tenant) GetID() string                  { return t.ID }
func (t *Tenant) GetName() string                { return t.Name }
func (t *Tenant) GetUpstream() string            { return t.Upstream }
func (t *Tenant) GetRateLimits() (int, int, int) { return t.RateLimitPS, t.RateLimitPM, t.RateLimitPD }

type Repository struct {
	db    *sql.DB
	mu    sync.RWMutex
	cache map[string]*Tenant
	ttl   time.Duration
	next  time.Time
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		db:    db,
		cache: make(map[string]*Tenant),
		ttl:   30 * time.Second,
	}
}

func (r *Repository) GetByAPIKeyHash(ctx context.Context, keyHash string) (*Tenant, error) {
	r.mu.RLock()
	if time.Now().Before(r.next) {
		t, ok := r.cache[keyHash]
		r.mu.RUnlock()
		if !ok {
			return nil, ErrNotFound
		}
		return t, nil
	}
	r.mu.RUnlock()

	if err := r.reload(ctx); err != nil {
		r.mu.RLock()
		t, ok := r.cache[keyHash]
		r.mu.RUnlock()
		if ok {
			return t, nil
		}
		return nil, fmt.Errorf("reload tenants: %w", err)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.cache[keyHash]
	if !ok {
		return nil, ErrNotFound
	}
	return t, nil
}

func (r *Repository) ListAll(ctx context.Context) ([]*Tenant, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, api_key_hash, upstream_url,
		       rate_limit_per_second, rate_limit_per_minute, rate_limit_per_day,
		       enabled, created_at
		FROM tenants ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []*Tenant
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(
			&t.ID, &t.Name, &t.APIKeyHash, &t.Upstream,
			&t.RateLimitPS, &t.RateLimitPM, &t.RateLimitPD,
			&t.Enabled, &t.CreatedAt,
		); err != nil {
			return nil, err
		}
		tenants = append(tenants, &t)
	}
	return tenants, rows.Err()
}

func (r *Repository) Create(ctx context.Context, t *Tenant) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tenants (id, name, api_key_hash, upstream_url,
		   rate_limit_per_second, rate_limit_per_minute, rate_limit_per_day, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, t.ID, t.Name, t.APIKeyHash, t.Upstream,
		t.RateLimitPS, t.RateLimitPM, t.RateLimitPD, t.Enabled,
	)
	if err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	r.invalidateCache()
	return nil
}

func (r *Repository) Update(ctx context.Context, t *Tenant) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE tenants SET name=$2, upstream_url=$3,
		  rate_limit_per_second=$4, rate_limit_per_minute=$5,
		  rate_limit_per_day=$6, enabled=$7
		WHERE id=$1
	`, t.ID, t.Name, t.Upstream,
		t.RateLimitPS, t.RateLimitPM, t.RateLimitPD, t.Enabled,
	)
	if err != nil {
		return fmt.Errorf("update tenant: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	r.invalidateCache()
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM tenants WHERE id=$1", id)
	r.invalidateCache()
	return err
}

func (r *Repository) reload(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, api_key_hash, upstream_url,
		       rate_limit_per_second, rate_limit_per_minute, rate_limit_per_day,
		       enabled, created_at
		FROM tenants WHERE enabled = true
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	newCache := make(map[string]*Tenant)
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(
			&t.ID, &t.Name, &t.APIKeyHash, &t.Upstream,
			&t.RateLimitPS, &t.RateLimitPM, &t.RateLimitPD,
			&t.Enabled, &t.CreatedAt,
		); err != nil {
			return err
		}
		newCache[t.APIKeyHash] = &t
	}
	if err := rows.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	r.cache = newCache
	r.next  = time.Now().Add(r.ttl)
	r.mu.Unlock()
	return nil
}

func (r *Repository) invalidateCache() {
	r.mu.Lock()
	r.next = time.Time{}
	r.mu.Unlock()
}
