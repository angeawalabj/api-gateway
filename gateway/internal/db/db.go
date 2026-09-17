package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	for i := range 10 {
		if err := db.Ping(); err == nil {
			return db, nil
		} else if i == 9 {
			return nil, fmt.Errorf("ping db after retries: %w", err)
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return db, nil
}

func Migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

const schema = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS tenants (
    id                    TEXT        PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name                  TEXT        NOT NULL,
    api_key_hash          TEXT        NOT NULL UNIQUE,
    upstream_url          TEXT        NOT NULL,
    rate_limit_per_second INTEGER     NOT NULL DEFAULT 100,
    rate_limit_per_minute INTEGER     NOT NULL DEFAULT 1000,
    rate_limit_per_day    INTEGER     NOT NULL DEFAULT 100000,
    enabled               BOOLEAN     NOT NULL DEFAULT true,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenants_api_key_hash
    ON tenants (api_key_hash) WHERE enabled = true;

CREATE TABLE IF NOT EXISTS request_logs (
    id          BIGSERIAL   PRIMARY KEY,
    tenant_id   TEXT        NOT NULL,
    method      TEXT        NOT NULL,
    path        TEXT        NOT NULL,
    status      INTEGER     NOT NULL,
    duration_ms INTEGER     NOT NULL,
    upstream    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_request_logs_tenant_time
    ON request_logs (tenant_id, created_at DESC);

INSERT INTO tenants (name, api_key_hash, upstream_url,
                     rate_limit_per_second, rate_limit_per_minute, rate_limit_per_day)
VALUES
    ('Demo Tenant A', 'sha256-demo-key-a', 'http://service-a:8081', 10, 100, 10000),
    ('Demo Tenant B', 'sha256-demo-key-b', 'http://service-b:8082', 5,  50,  5000),
    ('Demo Tenant C', 'sha256-demo-key-c', 'http://service-c:8083', 100, 1000, 100000)
ON CONFLICT (api_key_hash) DO NOTHING;
`
