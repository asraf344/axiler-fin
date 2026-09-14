.PHONY: help local-setup deploy logs test-search test-transfer test-auth test-failure test-rollback clean

help:
	@echo "Fintech Platform - Local Development Commands"
	@echo ""
	@echo "Setup & Teardown:"
	@echo "  make local-setup      - Initialize Swarm and deploy entire stack"
	@echo "  make clean            - Tear down all containers and volumes"
	@echo ""
	@echo "Testing:"
	@echo "  make test-auth        - Test authentication flow (get JWT token)"
	@echo "  make test-search      - Test search endpoint with tenant isolation"
	@echo "  make test-transfer    - Test transfer endpoint"
	@echo "  make test-failure     - Inject failure into transfer service"
	@echo "  make test-rollback    - Demonstrate automatic rollback"
	@echo ""
	@echo "Monitoring:"
	@echo "  make logs             - Tail all service logs"
	@echo "  make logs-edge        - Tail edge service logs"
	@echo "  make logs-transfer    - Tail transfer service logs"
	@echo ""
	@echo "URLs:"
	@echo "  Prometheus:  http://localhost:9090"
	@echo "  Grafana:     http://localhost:3000 (admin/admin)"
	@echo "  Kibana:      http://localhost:5601"
	@echo "  Jaeger:      http://localhost:16686"
	@echo "  Vault:       http://localhost:8200 (token: dev-token)"

local-setup: 
	@echo "Initializing fintech platform..."
	@echo ""
	@echo "Step 1: Building Docker images..."
	docker-compose build --no-cache
	@echo "✓ Images built"
	@echo ""
	@echo "Step 2: Starting services..."
	docker-compose up -d
	@echo "Waiting for services to become healthy..."
	@sleep 10
	@echo ""
	@echo "Step 3: Checking service health..."
	@docker-compose ps
	@echo ""
	@echo "✓ Platform initialized successfully!"
	@echo ""
	@echo "Services running:"
	@echo "  Edge (SSE):    http://localhost:8080/health"
	@echo "  Auth:          http://localhost:8081/health"
	@echo "  Search:        http://localhost:8082/health"
	@echo "  Transfer:      http://localhost:8083/health"
	@echo ""
	@echo "Monitoring:"
	@echo "  Prometheus:    http://localhost:9090/targets"
	@echo "  Grafana:       http://localhost:3000 (user: admin, pass: admin)"
	@echo "  Kibana:        http://localhost:5601"
	@echo "  Jaeger:        http://localhost:16686"

deploy:
	docker-compose up -d

test-auth:
	@echo "Testing authentication flow..."
	@echo ""
	@echo "Requesting JWT token for tenant 'alpha'..."
	@curl -s -X POST http://localhost:8080/auth/login \
		-H "Content-Type: application/json" \
		-d '{"api_key":"alpha_key_123","secret":"alpha_secret_456"}' | jq .
	@echo ""
	@echo "Tip: Copy the token above and use in subsequent requests"

test-search:
	@echo "Testing search endpoint with tenant 'alpha'..."
	@echo ""
	@echo "Step 1: Get token for alpha"
	@export TOKEN=$$(curl -s -X POST http://localhost:8080/auth/login \
		-H "Content-Type: application/json" \
		-d '{"api_key":"alpha_key_123","secret":"alpha_secret_456"}' | jq -r .token); \
	echo "Token: $$TOKEN"; \
	echo ""; \
	echo "Step 2: Search account (alpha can only see alpha accounts)"; \
	curl -s -X POST http://localhost:8080/api/v1/search \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $$TOKEN" \
		-d '{"account_id":"acc_alpha_001"}' | jq .

test-transfer:
	@echo "Testing transfer endpoint..."
	@echo ""
	@export TOKEN=$$(curl -s -X POST http://localhost:8080/auth/login \
		-H "Content-Type: application/json" \
		-d '{"api_key":"alpha_key_123","secret":"alpha_secret_456"}' | jq -r .token); \
	echo "Token: $$TOKEN"; \
	echo ""; \
	echo "Transferring $1500 from acc_alpha_001 to acc_alpha_002"; \
	curl -s -X POST http://localhost:8080/api/v1/transfer \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $$TOKEN" \
		-d '{"from_account":"acc_alpha_001","to_account":"acc_alpha_002","amount":1500}' | jq .

test-failure:
	@echo "Injecting failure into transfer service..."
	@curl -s -X POST http://localhost:8083/admin/simulate-failure | jq .
	@echo ""
	@echo "Transfer service health check should now fail."
	@echo "Check logs: docker logs fintech-transfer"
	@echo ""
	@echo "To verify automatic rollback in Swarm:"
	@echo "  docker service update --image registry/transfer:sha256:new transfer"
	@echo "  watch 'docker service ps transfer'"

test-rollback:
	@echo "Demonstrating rollback mechanism..."
	@echo ""
	@echo "Current transfer service status:"
	@curl -s http://localhost:8083/health | jq .status
	@echo ""
	@echo "To trigger rollback:"
	@echo "  1. Inject failure: make test-failure"
	@echo "  2. Wait 30 seconds for health checks to fail"
	@echo "  3. Docker Swarm automatically reverts to previous image digest"
	@echo ""
	@echo "Manual rollback:"
	@echo "  docker service rollback transfer"

logs:
	docker-compose logs -f

logs-edge:
	docker logs -f fintech-edge

logs-transfer:
	docker logs -f fintech-transfer

logs-search:
	docker logs -f fintech-search

logs-auth:
	docker logs -f fintech-auth

clean:
	@echo "Tearing down fintech platform..."
	docker-compose down -v
	@echo "✓ All containers and volumes removed"

ps:
	docker-compose ps

metrics:
	@echo "Querying metrics from Prometheus..."
	@curl -s 'http://localhost:9090/api/v1/query?query=up' | jq '.data.result[] | {job: .metric.job, instance: .metric.instance, value: .value[1]}'