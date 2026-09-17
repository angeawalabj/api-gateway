COMPOSE = docker compose -f infra/docker/docker-compose.yml
TF_DEV  = cd infra/terraform/environments/dev &&

.PHONY: help up down logs test test-bench build smoke tf-init tf-plan tf-apply tf-destroy clean

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS=":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ─── Dev local ────────────────────────────────────────────────────────────────

up: ## Démarre tous les services (Docker Compose)
	$(COMPOSE) up -d --build
	@echo ""
	@echo "  Gateway     → http://localhost:8080"
	@echo "  Admin UI    → http://localhost:3000"
	@echo "  Grafana     → http://localhost:3001  (admin/admin)"
	@echo "  Prometheus  → http://localhost:9091"
	@echo ""

down: ## Arrête les services
	$(COMPOSE) down

logs: ## Logs du gateway
	$(COMPOSE) logs -f gateway

logs-%: ## Logs d'un service : make logs-service-a
	$(COMPOSE) logs -f $*

# ─── Tests ────────────────────────────────────────────────────────────────────

test: ## Tests unitaires Go (avec race detector)
	cd gateway && go test ./... -v -race -count=1

test-bench: ## Benchmarks Go (latence ajoutée par le gateway)
	cd gateway && go test ./... -bench=. -benchtime=2s -run=^$ | head -30

# ─── Build ────────────────────────────────────────────────────────────────────

build: ## Build le binaire Go Linux amd64
	cd gateway && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -ldflags="-w -s" -o gateway-linux-amd64 ./cmd/gateway
	@echo "Binary: gateway/gateway-linux-amd64"

build-dev: ## Lance le gateway en mode dev local
	cd gateway && DEV_MODE=true go run ./cmd/gateway

# ─── Smoke tests ──────────────────────────────────────────────────────────────

smoke: ## Smoke tests contre le gateway local
	bash scripts/smoke-test.sh

# ─── Terraform AWS ────────────────────────────────────────────────────────────

tf-init: ## Initialise Terraform
	$(TF_DEV) terraform init

tf-plan: ## Plan Terraform
	$(TF_DEV) terraform plan

tf-apply: ## Déploie sur AWS Free Tier
	$(TF_DEV) terraform apply

tf-destroy: ## ⚠️ Détruit toute l'infra AWS
	$(TF_DEV) terraform destroy

tf-output: ## Outputs Terraform
	$(TF_DEV) terraform output

# ─── Utilitaires ──────────────────────────────────────────────────────────────

clean: ## Nettoie les artefacts de build
	rm -f gateway/gateway-linux-amd64
	find . -name "*.test" -delete
