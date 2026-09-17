# API Gateway Multi-tenant — Go + AWS + Terraform

Gateway HTTP multi-tenant en Go avec rate limiting Redis sliding window, auth JWT, observabilité Prometheus/Grafana et déploiement AWS Free Tier via Terraform.

## Stack

| Couche | Technologie |
|--------|-------------|
| Gateway core | Go 1.22 · `net/http` · `httputil.ReverseProxy` |
| Rate limiting | Redis sliding window (Lua atomique) · 3 fenêtres : /s, /min, /jour |
| Auth | JWT RS256 (middleware configurable) |
| Storage | PostgreSQL 16 (tenants, routes, logs) |
| Cache tenant | In-memory 30s TTL + stale-on-error |
| Observabilité | Prometheus + Grafana |
| Admin UI | React + Vite |
| IaC | Terraform · AWS Free Tier (EC2 t2.micro + RDS + ElastiCache) |
| CI/CD | GitHub Actions · build Go · S3 · SSM deploy |

## Démarrage local

```bash
# Clone + démarrage
git clone https://github.com/ton-user/api-gateway
cd api-gateway
cp .env.example .env

make up       # démarre tous les services Docker

# Accès
# Gateway     → http://localhost:8080
# Admin UI    → http://localhost:3000
# Grafana     → http://localhost:3001  (admin/admin)
# Prometheus  → http://localhost:9091

make smoke    # smoke tests
make test     # tests unitaires Go (race detector)
```

## Tests Go

```bash
make test          # 15 tests unitaires (ratelimit + middleware)
make test-bench    # lance `go test -bench` (aucun benchmark écrit pour l'instant)
```

## Middleware chain

```
Request
  → Recovery        (panic → 500)
  → Logger          (JSON structuré, durée, tenant)
  → CORS            (headers cross-origin pour l'admin UI)
  → Metrics         (Prometheus counter + histogram)
  → TenantResolve   (X-API-Key → tenant PostgreSQL + cache 30s)
  → RateLimit       (Redis sliding window, 3 fenêtres)
  → ReverseProxy    (httputil → upstream, inject X-Tenant-ID)
Response
```

## Déploiement AWS Free Tier

```bash
# Prérequis : aws CLI configuré, keypair EC2, bucket S3
cp infra/terraform/environments/dev/terraform.tfvars.example \
   infra/terraform/environments/dev/terraform.tfvars
# Éditer les variables (key_name, db_password, s3_bucket)

make tf-init
make tf-apply     # EC2 t2.micro + RDS db.t3.micro + ElastiCache cache.t3.micro

# Coût : ~0-2$/mois (Free Tier 12 mois)

# Fin de démo — destroy en une commande
make tf-destroy   # coût → $0
```

## Architecture

```
                    ┌─────────────────────────────┐
Clients ──────────► │   API Gateway (Go :8080)    │
X-API-Key           │   Recovery → Logger → CORS  │
                    │   → Metrics → TenantResolve │
                    │   → RateLimit → Proxy        │
                    └────────────┬────────────────┘
                                 │
                    ┌────────────▼────────────────┐
                    │   Services Upstream          │
                    │   service-a:8081             │
                    │   service-b:8082             │
                    │   service-c:8083             │
                    └─────────────────────────────┘

Storage:
  Redis      ← rate limit counters (sliding window)
  PostgreSQL ← tenants, routes, request logs

Observabilité:
  Prometheus ← /metrics :9090
  Grafana    ← dashboards :3001
```

## ADR

| # | Sujet | Décision |
|---|-------|----------|
| [ADR-0001](docs/adr/ADR-0001-go-gateway.md) | Langage gateway | Go — goroutines, stdlib net/http |
| [ADR-0002](docs/adr/ADR-0002-rate-limiting.md) | Rate limiting | Sliding window Redis (Lua atomique) |
| [ADR-0003](docs/adr/ADR-0003-aws-terraform.md) | Déploiement | AWS Free Tier + Terraform |
