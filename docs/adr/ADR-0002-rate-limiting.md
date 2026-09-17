# ADR-0002 — Rate limiting : sliding window Redis

| Champ  | Valeur |
|--------|--------|
| Statut | **Accepté** |
| Date   | 2026-07-07 |

## Décision

**Sliding window counter** avec script Lua atomique Redis.

```lua
local current = redis.call("INCR", KEYS[1])
if current == 1 then redis.call("PEXPIRE", KEYS[1], window_secs * 2000) end
local prev    = tonumber(redis.call("GET", KEYS[2]) or "0")
local weight  = elapsed_ms / (window_secs * 1000)
local estimated = prev * (1 - weight) + current
return {current, estimated > limit and 1 or 0, prev}
```

## Clés Redis

```
rl:{tenant_id}:per_second:{unix_second}
rl:{tenant_id}:per_minute:{unix_minute}
rl:{tenant_id}:per_day:{unix_day}
```

## Headers retournés

```
X-RateLimit-Limit:     1000
X-RateLimit-Remaining: 847
X-RateLimit-Reset:     1720000060
Retry-After:           1   (uniquement si 429)
```

## Conséquences

- **Positif** : Atomique — pas de TOCTOU
- **Positif** : Trois fenêtres vérifiées sur la même requête
- **Négatif** : Dépendance Redis obligatoire
- **Décision** : Si Redis down → fail-open (requête autorisée, log d'alerte)
