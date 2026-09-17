# ADR-0001 — Go comme langage du gateway

| Champ  | Valeur |
|--------|--------|
| Statut | **Accepté** |
| Date   | 2026-07-07 |
| Tags   | go, performance, gateway |

## Contexte

Le gateway est le point d'entrée unique — latence ajoutée < 2ms p99, milliers de connexions simultanées.

## Options évaluées

| Critère | Go | Node/Fastify | Python/FastAPI |
|---------|-----|-------------|----------------|
| Latence overhead | < 0.5ms | ~1-2ms | ~3-5ms |
| Connexions simultanées | goroutines (millions) | libuv (milliers) | asyncio (milliers) |
| net/http + ReverseProxy | ✅ built-in | ❌ externe | ❌ externe |
| Binaire statique | ✅ 8MB | ❌ node_modules | ❌ venv |
| Idiomatique gateway | ✅ Kong/Traefik/Envoy | ⚠️ | ❌ |

## Décision

**Go** — goroutines (~4KB stack), `net/http` + `httputil.ReverseProxy` sans framework, binaire statique 8MB pour EC2.

## Chaîne de middleware

```
Request → Recovery → Logger → CORS → Metrics → TenantResolve → RateLimit → ReverseProxy → Response
```

## Conséquences

- **Positif** : Binaire unique, `scp + systemctl restart` pour déployer
- **Positif** : `go test -bench` intégré pour mesurer la latence ajoutée
- **Négatif** : Gestion manuelle des erreurs, requêtes SQL à la main
