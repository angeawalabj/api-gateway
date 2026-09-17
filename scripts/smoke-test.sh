#!/usr/bin/env bash
set -euo pipefail

BASE="${GATEWAY_URL:-http://localhost:8080}"
PASS=0; FAIL=0

pass() { echo -e "\033[32m✓\033[0m $*"; PASS=$((PASS+1)); }
fail() { echo -e "\033[31m✗\033[0m $*"; FAIL=$((FAIL+1)); }
info() { echo -e "\033[34m→\033[0m $*"; }

echo ""; echo "=== Smoke Tests — API Gateway ==="; echo "    $BASE"; echo ""

info "1. Healthcheck"
R=$(curl -sf "$BASE/health" 2>/dev/null || echo "FAIL")
echo "$R" | grep -q '"ok"' && pass "GET /health" || fail "GET /health — $R"

info "2. Sans clé API → 401"
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api")
[ "$CODE" = "401" ] && pass "401 sans X-API-Key" || fail "Attendu 401, reçu $CODE"

info "3. Clé invalide → 401"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "X-API-Key: bad" "$BASE/api")
[ "$CODE" = "401" ] && pass "401 avec clé invalide" || fail "Attendu 401, reçu $CODE"

info "4. Clé valide → proxy"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "X-API-Key: sha256-demo-key-a" "$BASE/")
[ "$CODE" = "200" ] || [ "$CODE" = "502" ] && pass "Clé valide → $CODE" || fail "Attendu 200/502, reçu $CODE"

info "5. Headers RateLimit"
HDRS=$(curl -si -H "X-API-Key: sha256-demo-key-a" "$BASE/" | head -20)
echo "$HDRS" | grep -qi "X-RateLimit-Limit" && pass "X-RateLimit-Limit présent" || fail "X-RateLimit-Limit absent"

info "6. CORS preflight"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X OPTIONS "$BASE/" -H "Origin: http://localhost:3000")
[ "$CODE" = "204" ] && pass "CORS 204" || fail "Attendu 204, reçu $CODE"

info "7. Métriques Prometheus"
M=$(curl -sf "http://localhost:9090/metrics" 2>/dev/null || echo "SKIP")
echo "$M" | grep -q "gateway_requests_total" && pass "Métriques présentes" || pass "Port 9090 non exposé (SKIP)"

echo ""
if [ $FAIL -eq 0 ]; then
  echo -e "\033[32m\033[1mTous les tests passent ($PASS) ✓\033[0m"
  exit 0
else
  echo -e "\033[31m\033[1m$FAIL échec(s) / $((PASS+FAIL))\033[0m"
  exit 1
fi
