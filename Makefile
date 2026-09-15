.PHONY: help swarm-init swarm-deploy swarm-update swarm-rollback swarm-scale swarm-status swarm-monitor swarm-clean swarm-logs swarm-test

# Colors
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m

STACK_NAME ?= fintech
ENV_FILE ?= .env.swarm

GORELEASER_VERSION ?= 2.18.1
TOOL_DIR ?= .tool
GORELEASER="$(TOOL_DIR)/goreleaser"




help:
	@echo "$(BLUE)Fintech Platform - Docker Swarm Commands$(NC)"
	@echo ""
	@echo "$(YELLOW)Setup & Initialization:$(NC)"
	@echo "  make swarm-init              - Initialize Docker Swarm cluster"
	@echo "  make swarm-deploy            - Deploy stack to Swarm"
	@echo "  make swarm-deploy ENV_FILE=prod.env - Deploy with custom env file"
	@echo ""
	@echo "$(YELLOW)Service Management:$(NC)"
	@echo "  make swarm-update SERVICE=transfer IMAGE=registry.com/transfer:sha256:abc123"
	@echo "  make swarm-rollback SERVICE=transfer - Rollback service to previous version"
	@echo "  make swarm-scale SERVICE=edge REPLICAS=5 - Scale service"
	@echo ""
	@echo "$(YELLOW)Monitoring & Debugging:$(NC)"
	@echo "  make swarm-status            - Show current stack status"
	@echo "  make swarm-monitor           - Interactive monitoring (hit 'q' to exit)"
	@echo "  make swarm-logs SERVICE=transfer - Show service logs"
	@echo "  make swarm-logs-follow SERVICE=transfer - Follow service logs"
	@echo "  make swarm-health SERVICE=transfer - Show service health details"
	@echo "  make swarm-tasks             - Show all tasks in stack"
	@echo ""
	@echo "$(YELLOW)Testing & Validation:$(NC)"
	@echo "  make swarm-test-auth         - Test authentication"
	@echo "  make swarm-test-search       - Test search endpoint"
	@echo "  make swarm-test-transfer     - Test transfer endpoint"
	@echo "  make swarm-test-failover     - Test service failover"
	@echo ""
	@echo "$(YELLOW)Cluster Management:$(NC)"
	@echo "  make swarm-nodes             - List all nodes"
	@echo "  make swarm-node-label NODE_ID - Label a node for service placement"
	@echo "  make swarm-drain NODE_ID     - Drain node for maintenance"
	@echo "  make swarm-restore NODE_ID   - Restore node to active"
	@echo "  make swarm-clean             - Remove all services and leave Swarm"
	@echo ""
	@echo "$(YELLOW)URLs (after deployment):$(NC)"
	@echo "  Edge:        http://localhost:8080/health"
	@echo "  Prometheus:  http://localhost:9090"
	@echo "  Grafana:     http://localhost:3000 (admin/admin)"
	@echo "  Kibana:      http://localhost:5601"
	@echo "  Jaeger:      http://localhost:16686"
	@echo ""
	@echo "$(YELLOW)Examples:$(NC)"
	@echo "  make swarm-init"
	@echo "  make swarm-deploy"
	@echo "  make swarm-status"
	@echo "  make swarm-scale SERVICE=transfer REPLICAS=5"
	@echo "  make swarm-update SERVICE=transfer IMAGE=myregistry.com/transfer:sha256:abc123..."
	@echo "  make swarm-rollback SERVICE=transfer"

# =============================================================================
# Swarm Initialization
# =============================================================================

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: fmt ## Run go vet against code.
	go vet ./...


UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Darwin)
	GOOS := Darwin
else ifeq ($(UNAME_S),Linux)
	GOOS := Linux
else
	$(error Unsupported OS: $(UNAME_S))
endif

ifeq ($(UNAME_M),x86_64)
	GOARCH := x86_64
else ifeq ($(UNAME_M),amd64)
	GOARCH := x86_64
else ifeq ($(UNAME_M),arm64)
	GOARCH := arm64
else ifeq ($(UNAME_M),aarch64)
	GOARCH := arm64
else
	$(error Unsupported architecture: $(UNAME_M))
endif

GORELEASER_ARCHIVE := goreleaser_$(GOOS)_$(GOARCH).tar.gz
GORELEASER_URL := https://github.com/goreleaser/goreleaser/releases/download/v$(GORELEASER_VERSION)/$(GORELEASER_ARCHIVE)
## https://github.com/goreleaser/goreleaser/releases/download/v2.18.1/goreleaser_Darwin_arm64.tar.gz

.PHONY: goreleaser
goreleaser:
	@mkdir -p $(TOOL_DIR)
	@if [ ! -x "$(GORELEASER)" ]; then \
		echo "Downloading GoReleaser $(GORELEASER_VERSION) for $(GOOS)/$(GOARCH)..."; \
		curl -fsSL "$(GORELEASER_URL)" -o /tmp/$(GORELEASER_ARCHIVE); \
		tar -xzf /tmp/$(GORELEASER_ARCHIVE) -C $(TOOL_DIR) goreleaser; \
		rm /tmp/$(GORELEASER_ARCHIVE); \
		chmod +x "$(GORELEASER)"; \
	fi

snapshot:
	@echo "$(YELLOW)Building snapshot release...$(NC)"
	@$(GORELEASER) release --clean --snapshot --skip=publish

build-images:
	@echo "$(YELLOW)Building Docker images...$(NC)"
	@chmod +x ./infra/scripts/build-images.sh
	@./infra/scripts/build-images.sh

swarm-init:
	@echo "$(YELLOW)Initializing Docker Swarm cluster...$(NC)"
	@chmod +x ./infra/scripts/swarm-init.sh
	@./infra/scripts/swarm-init.sh

swarm-init-prod:
	@echo "$(YELLOW)Initializing Docker Swarm for PRODUCTION...$(NC)"
	@echo "$(RED)This will enable Swarm in production mode$(NC)"
	@read -p "Continue? (y/n) " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		chmod +x ./infra/scripts/swarm-init.sh; \
		./infra/scripts/swarm-init.sh production; \
	fi

# =============================================================================
# Stack Deployment
# =============================================================================

swarm-deploy:
	@echo "$(YELLOW)Deploying stack to Swarm...$(NC)"
	@if [ ! -f "$(ENV_FILE)" ]; then \
		echo "$(RED)$(ENV_FILE) not found, using defaults$(NC)"; \
		echo "Copy .env.swarm.example to $(ENV_FILE) and update"; \
	fi
	@chmod +x ./infra/scripts/swarm-deploy.sh
	@./infra/scripts/swarm-deploy.sh "$(ENV_FILE)"

swarm-deploy-prod:
	@echo "$(RED)PRODUCTION DEPLOYMENT$(NC)"
	@read -p "Continue with production deployment? (yes/no) " -r confirm; \
	if [ "$$confirm" = "yes" ]; then \
		make swarm-deploy ENV_FILE=.env.swarm.prod; \
	else \
		echo "Deployment cancelled"; \
	fi

swarm-redeploy: swarm-clean swarm-deploy
	@echo "$(GREEN)Stack redeployed$(NC)"

# =============================================================================
# Service Updates & Rollbacks
# =============================================================================

swarm-update:
	@if [ -z "$(SERVICE)" ] || [ -z "$(IMAGE)" ]; then \
		echo "$(YELLOW)Usage: make swarm-update SERVICE=<name> IMAGE=<image:digest>$(NC)"; \
		echo ""; \
		echo "Examples:"; \
		echo "  make swarm-update SERVICE=transfer IMAGE=myregistry.com/transfer:sha256:abc123..."; \
		echo "  make swarm-update SERVICE=edge IMAGE=myregistry.com/edge:v2.1.0"; \
	else \
		chmod +x ./infra/scripts/swarm-update.sh; \
		./infra/scripts/swarm-update.sh "$(STACK_NAME)" "$(SERVICE)" "$(IMAGE)"; \
	fi

swarm-rollback:
	@if [ -z "$(SERVICE)" ]; then \
		echo "$(YELLOW)Usage: make swarm-rollback SERVICE=<name>$(NC)"; \
		echo ""; \
		echo "Examples:"; \
		echo "  make swarm-rollback SERVICE=transfer"; \
		echo "  make swarm-rollback SERVICE=edge"; \
		echo ""; \
		echo "Available services:"; \
		docker stack services $(STACK_NAME) 2>/dev/null | tail -n +2 | awk '{print "  " $$2}'; \
	else \
		chmod +x ./infra/scripts/swarm-rollback.sh; \
		./infra/scripts/swarm-rollback.sh "$(STACK_NAME)" "$(SERVICE)"; \
	fi

# =============================================================================
# Scaling
# =============================================================================

swarm-scale:
	@if [ -z "$(SERVICE)" ] || [ -z "$(REPLICAS)" ]; then \
		echo "$(YELLOW)Usage: make swarm-scale SERVICE=<name> REPLICAS=<count>$(NC)"; \
		echo ""; \
		echo "Examples:"; \
		echo "  make swarm-scale SERVICE=transfer REPLICAS=5"; \
		echo "  make swarm-scale SERVICE=edge REPLICAS=10"; \
	else \
		echo "Scaling $(STACK_NAME)_$(SERVICE) to $(REPLICAS) replicas..."; \
		docker service scale $(STACK_NAME)_$(SERVICE)=$(REPLICAS); \
		echo ""; \
		echo "$(GREEN)✓ Scaling initiated$(NC)"; \
		echo "Monitor progress: make swarm-status"; \
	fi

# =============================================================================
# Monitoring & Status
# =============================================================================

swarm-status:
	@echo "$(BLUE)=== Stack Status ($(STACK_NAME)) ===$(NC)"
	@docker stack services $(STACK_NAME) --format "table {{.Name}}\t{{.Mode}}\t{{.Replicas}}\t{{.Image}}"
	@echo ""
	@echo "$(BLUE)=== Service Health ===$(NC)"
	@docker stack ps $(STACK_NAME) --format "table {{.Name}}\t{{.Node}}\t{{.DesiredState}}\t{{.CurrentState}}" | head -20

swarm-monitor:
	@echo "$(YELLOW)Starting interactive monitor...$(NC)"
	@chmod +x ./infra/scripts/swarm-monitor.sh
	@./infra/scripts/swarm-monitor.sh "$(STACK_NAME)"

swarm-quick-status:
	@clear
	@echo "$(BLUE)=== Quick Status Report ===$(NC)"
	@echo ""
	@echo "Cluster nodes:"
	@docker node ls --format "table {{.ID}}\t{{.Hostname}}\t{{.Status}}\t{{.Availability}}" | head -6
	@echo ""
	@echo "Stack services:"
	@docker stack services $(STACK_NAME) --format "table {{.Name}}\t{{.Replicas}}" 2>/dev/null | head -6
	@echo ""
	@echo "Task summary:"
	@docker stack ps $(STACK_NAME) --format "table {{.DesiredState}}\t{{.CurrentState}}" | tail -n +2 | sort | uniq -c

# =============================================================================
# Logs
# =============================================================================

swarm-logs:
	@if [ -z "$(SERVICE)" ]; then \
		echo "$(YELLOW)Usage: make swarm-logs SERVICE=<name> [LINES=50]$(NC)"; \
		echo ""; \
		echo "Examples:"; \
		echo "  make swarm-logs SERVICE=transfer"; \
		echo "  make swarm-logs SERVICE=edge LINES=100"; \
	else \
		chmod +x ./infra/scripts/swarm-monitor.sh; \
		./infra/scripts/swarm-monitor.sh "$(STACK_NAME)" logs "$(SERVICE)" "$(LINES)"; \
	fi

swarm-logs-follow:
	@if [ -z "$(SERVICE)" ]; then \
		echo "$(YELLOW)Usage: make swarm-logs-follow SERVICE=<name>$(NC)"; \
	else \
		echo "Following logs for $(STACK_NAME)_$(SERVICE) (Ctrl+C to stop)..."; \
		docker service logs -f $(STACK_NAME)_$(SERVICE); \
	fi

swarm-logs-all:
	@echo "Collecting logs from all services (last 50 lines)..."
	@docker stack ps $(STACK_NAME) --format "{{.Name}}" | sort -u | while read service; do \
		echo ""; \
		echo "$(BLUE)=== $$service ===$(NC)"; \
		docker service logs $$service -n 50 2>/dev/null | tail -20; \
	done

# =============================================================================
# Service Health & Details
# =============================================================================

swarm-health:
	@if [ -z "$(SERVICE)" ]; then \
		echo "$(YELLOW)Usage: make swarm-health SERVICE=<name>$(NC)"; \
	else \
		chmod +x ./infra/scripts/swarm-monitor.sh; \
		./infra/scripts/swarm-monitor.sh "$(STACK_NAME)" info "$(SERVICE)"; \
	fi

swarm-tasks:
	@echo "$(BLUE)=== Tasks in $(STACK_NAME) ===$(NC)"
	@docker stack ps $(STACK_NAME) --no-trunc --format "table {{.ID}}\t{{.Name}}\t{{.Node}}\t{{.DesiredState}}\t{{.CurrentState}}\t{{.Error}}" | head -30

swarm-failed-tasks:
	@echo "$(BLUE)=== Failed/Stopped Tasks ===$(NC)"
	@docker stack ps $(STACK_NAME) --filter "desired-state=shutdown" --format "table {{.Name}}\t{{.Node}}\t{{.CurrentState}}"

# =============================================================================
# Cluster Management
# =============================================================================

swarm-nodes:
	@echo "$(BLUE)=== Swarm Nodes ===$(NC)"
	@docker node ls --format "table {{.ID}}\t{{.Hostname}}\t{{.Status}}\t{{.Availability}}\t{{.ManagerStatus}}"

swarm-node-info:
	@if [ -z "$(NODE)" ]; then \
		echo "$(YELLOW)Usage: make swarm-node-info NODE=<node-id>$(NC)"; \
	else \
		echo "$(BLUE)=== Node Information ===$(NC)"; \
		docker node inspect $(NODE) | jq '.[0] | {ID, Description, Status, Spec}'; \
	fi

swarm-node-label:
	@if [ -z "$(NODE)" ] || [ -z "$(LABEL)" ]; then \
		echo "$(YELLOW)Usage: make swarm-node-label NODE=<node-id> LABEL=<key>=<value>$(NC)"; \
		echo ""; \
		echo "Examples:"; \
		echo "  make swarm-node-label NODE=abc123 LABEL=service=transfer"; \
		echo "  make swarm-node-label NODE=abc123 LABEL=stability=high"; \
	else \
		docker node update --label-add $(LABEL) $(NODE); \
		echo "$(GREEN)✓ Label added to node $(NODE)$(NC)"; \
	fi

swarm-drain:
	@if [ -z "$(NODE)" ]; then \
		echo "$(YELLOW)Usage: make swarm-drain NODE=<node-id>$(NC)"; \
	else \
		echo "Draining node $(NODE) for maintenance..."; \
		docker node update --availability drain $(NODE); \
		echo "Tasks will be rescheduled to other nodes..."; \
	fi

swarm-restore:
	@if [ -z "$(NODE)" ]; then \
		echo "$(YELLOW)Usage: make swarm-restore NODE=<node-id>$(NC)"; \
	else \
		echo "Restoring node $(NODE) to active..."; \
		docker node update --availability active $(NODE); \
		echo "$(GREEN)✓ Node restored$(NC)"; \
	fi

# =============================================================================
# Testing
# =============================================================================

swarm-test-auth:
	@echo "$(YELLOW)Testing authentication endpoint...$(NC)"
	@curl -s -X POST http://localhost:8080/auth/login \
		-H "Content-Type: application/json" \
		-d '{"api_key":"alpha_key_123","secret":"alpha_secret_456"}' | jq .
	@echo ""

swarm-test-search:
	@echo "$(YELLOW)Testing search endpoint...$(NC)"
	@TOKEN=$$(curl -s -X POST http://localhost:8080/auth/login \
		-H "Content-Type: application/json" \
		-d '{"api_key":"alpha_key_123","secret":"alpha_secret_456"}' | jq -r .token); \
	curl -s -X POST http://localhost:8080/api/v1/search \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $$TOKEN" \
		-d '{"account_id":"acc_alpha_001"}' | jq .
	@echo ""

swarm-test-transfer:
	@echo "$(YELLOW)Testing transfer endpoint...$(NC)"
	@TOKEN=$$(curl -s -X POST http://localhost:8080/auth/login \
		-H "Content-Type: application/json" \
		-d '{"api_key":"alpha_key_123","secret":"alpha_secret_456"}' | jq -r .token); \
	curl -s -X POST http://localhost:8080/api/v1/transfer \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $$TOKEN" \
		-d '{"from_account":"acc_alpha_001","to_account":"acc_alpha_002","amount":1500}' | jq .
	@echo ""

swarm-test-failover:
	@echo "$(YELLOW)Testing service failover...$(NC)"
	@echo "Getting transfer service info..."
	@docker service ps $(STACK_NAME)_transfer | head -5
	@echo ""
	@echo "Killing one task to trigger failover..."
	@TASK=$$(docker stack ps $(STACK_NAME)_transfer --format "{{.ID}}" | head -1); \
	if [ -n "$$TASK" ]; then \
		CONTAINER=$$(docker ps --filter "label=com.docker.swarm.task.id=$$TASK" -q | head -1); \
		if [ -n "$$CONTAINER" ]; then \
			echo "Stopping container $$CONTAINER..."; \
			docker stop $$CONTAINER; \
			echo ""; \
			echo "Task status will update momentarily:"; \
			sleep 5; \
			docker service ps $(STACK_NAME)_transfer | head -5; \
		fi; \
	fi

# =============================================================================
# Cleanup
## docker swarm leave --force; \
# =============================================================================

swarm-clean:
	@echo "$(RED)Removing stack and leaving Swarm...$(NC)"
	@read -p "Continue? (y/n) " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		echo "Removing stack $(STACK_NAME)..."; \
		docker stack rm $(STACK_NAME); \
		echo "Waiting for services to terminate..."; \
		sleep 10; \
		echo "$(GREEN)✓ Swarm cleanup complete$(NC)"; \
	fi

swarm-prune:
	@echo "$(YELLOW)Pruning unused Docker resources...$(NC)"
	@docker system prune -f --volumes
	@echo "$(GREEN)✓ Pruned$(NC)"

# =============================================================================
# Convenience Targets
# =============================================================================

swarm-bootstrap: swarm-init swarm-deploy
	@echo "$(GREEN)✓ Bootstrap complete$(NC)"

swarm-restart: swarm-clean swarm-deploy
	@echo "$(GREEN)✓ Restart complete$(NC)"

swarm-help-advanced:
	@echo "$(BLUE)Advanced Swarm Operations$(NC)"
	@echo ""
	@echo "Update strategy (all at once):"
	@echo "  docker service update --update-parallelism 3 --update-delay 0s $(STACK_NAME)_transfer"
	@echo ""
	@echo "Force update (ignore errors):"
	@echo "  docker service update --force $(STACK_NAME)_edge"
	@echo ""
	@echo "Service labels:"
	@echo "  docker service update --label-add version=v2.1.0 $(STACK_NAME)_transfer"
	@echo ""
	@echo "Resource limits:"
	@echo "  docker service update --limit-cpu 2 --limit-memory 1G $(STACK_NAME)_transfer"
	@echo ""
	@echo "Environment variables:"
	@echo "  docker service update -e LOG_LEVEL=debug $(STACK_NAME)_edge"
