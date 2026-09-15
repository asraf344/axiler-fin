# Quick Start Guide

## 5-Minute Setup

```bash
# Clone or navigate to the project
cd axiler-fin

# Start everything
make  build-images swarm-init  swarm-deploy

# Wait for the "Platform initialized successfully!" message
```

## First Test (2 minutes)

```bash
# Terminal 1: check status
make swarm-status

# Terminal 2: Run a test
make swarm-test-auth

# You'll see:
# {
#   "token": "eyJhbGc...",
#   "expires_at": "2026-09-15T10:30:00Z",
#   "tenant_id": "alpha"
# }
```

## Understanding Tenant Isolation (3 minutes)

```bash
# Get alpha's token
TOKEN_A=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"api_key":"alpha_key_123","secret":"alpha_secret_456"}' | jq -r .token)

# Alpha CAN access their own account
curl -s -X POST http://localhost:8080/api/v1/search \
  -H "Authorization: Bearer $TOKEN_A" \
  -d '{"account_id":"acc_alpha_001"}' | jq .

# Alpha CANNOT access beta's account
curl -s -X POST http://localhost:8080/api/v1/search \
  -H "Authorization: Bearer $TOKEN_A" \
  -d '{"account_id":"acc_beta_001"}' | jq .

# Result: 403 Forbidden (security violation logged)
```

## Security Controls Demo (5 minutes)

### 1. Rate Limiting

```bash
# Check rate limits for each tenant
curl http://localhost:8080/admin/ratelimit-status/alpha | jq .
curl http://localhost:8080/admin/ratelimit-status/beta | jq .
curl http://localhost:8080/admin/ratelimit-status/gamma | jq .

# Output shows:
# alpha: 1000 req/min
# beta: 500 req/min
# gamma: 250 req/min

# To test: hit any endpoint 251+ times as gamma to trigger rate limit
```

### 2. Health Checks & Rollback

```bash
# Enable failure mode in transfer service
make test-failure

# Health check will fail within 10 seconds
docker service logs fintech_transfer | grep -i health

# Disable failure mode
make test-failure  # Toggle off

# Service recovers immediately
```

### 3. Structured Logging

```bash
# Find all logs for a specific trace ID
docker  service logs fintech_edge | grep trace_123456789

# Or in Kibana: http://localhost:5601
# Search: trace_id: "trace_123456789"
```

## Monitoring & Observability

### Prometheus (Metrics)
```bash
# Open http://localhost:9090

# Useful queries:
# - rate(http_requests_total[5m]) by (tenant)
# - histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
# - edge_ratelimit_hits_total
```

### Grafana (Dashboard)
```bash
# Open http://localhost:3000
# Username: admin
# Password: admin

# Pre-built dashboards:
# - Request rates by tenant
# - Error rates and latency
# - Service health status
```

### Kibana (Logs)
```bash
# Open http://localhost:5601
# All structured logs from all services
# Filter by tenant_id, trace_id, service, or level
```

### Jaeger (Traces)
```bash
# Open http://localhost:16686
# View full request traces across services
# Shows latency breakdown by service
```

## Testing the API

### Test Data Available

**Tenant Alpha:**
- API Key: `alpha_key_123`
- Secret: `alpha_secret_456`
- Accounts: `acc_alpha_001` ($50k), `acc_alpha_002` ($100k)

**Tenant Beta:**
- API Key: `beta_key_789`
- Secret: `beta_secret_012`
- Accounts: `acc_beta_001` ($250k)

**Tenant Gamma:**
- API Key: `gamma_key_345`
- Secret: `gamma_secret_678`
- Accounts: `acc_gamma_001` ($5k)

### Full Request/Response Example

```bash
# 1. Authenticate
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"api_key":"alpha_key_123","secret":"alpha_secret_456"}' \
  | jq -r .token)

echo "Token: $TOKEN"

# 2. Search account
curl -s -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"account_id":"acc_alpha_001"}' | jq .

# Expected response:
# {
#   "account_id": "acc_alpha_001",
#   "account_name": "Alice Corp",
#   "balance": 50000.0,
#   "tenant_id": "alpha",
#   "status": "active",
#   "last_updated": "2024-09-13T..."
# }

# 3. Transfer funds
curl -s -X POST http://localhost:8080/api/v1/transfer \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "from_account": "acc_alpha_001",
    "to_account": "acc_alpha_002",
    "amount": 1500,
    "description": "Test transfer"
  }' | jq .

# Expected response:
# {
#   "transfer_id": "txn_1726343400000000000",
#   "status": "completed",
#   "from_account": "acc_alpha_001",
#   "to_account": "acc_alpha_002",
#   "amount": 1500,
#   "created_at": "2026-09-14T...",
#   "message": "Transfer successful"
# }
```

## What's Happening Behind the Scenes

### Request Flow

```
curl request
    ↓
Edge Service (Port 8080)
    ├─ Validate TLS cert
    ├─ Check rate limit
    ├─ Scan for secrets
    ├─ Detect abuse patterns
    └─ Log request with trace_id
    ↓
Auth/Search/Transfer Service
    ├─ Validate JWT token
    ├─ Extract tenant from claims
    ├─ Enforce tenant isolation
    └─ Process request
    ↓
Vault (Optional in demo)
    └─ Get secrets (not used in demo, but wired up)
    ↓
Response
    ├─ Include trace_id in headers
    ├─ Record metrics
    └─ Log to structured JSON
    ↓
Observable: Prometheus → Grafana, Elasticsearch → Kibana, Jaeger
```

### Tenant Isolation Check

Every service enforces:
```go
// Critical security check in search-service/main.go
if account.TenantID != reqCtx.TenantID {
    log.Error("Attempted to access account from different tenant - SECURITY VIOLATION")
    return 403 Forbidden
}
```

### Metrics Collection

Each service exports Prometheus metrics:
- `http_requests_total{tenant="alpha", endpoint="/search", status="200"}`
- `http_request_duration_seconds{tenant="alpha"}`
- `edge_auth_failures_total{reason="invalid_credentials"}`
- `edge_ratelimit_hits_total{tenant="gamma"}`
- `service_health_status{service="transfer"}`

### Structured Logging

Every log includes:
```json
{
  "timestamp": "2026-09-14T10:30:45.123Z",
  "level": "info",
  "tenant_id": "alpha",
  "trace_id": "trace_123456",
  "request_id": "req_789",
  "user_id": "alpha_user",
  "service": "search",
  "release_ver": "1.0.0",
  "message": "Search request succeeded"
}
```

## File Structure

```
fintech-platform/
├── cmd/
│   ├── edge-service/main.go (API Gateway)
│   ├── auth-service/main.go (JWT Issuer)
│   ├── search-service/main.go (Account Lookup)
│   └── transfer-service/main.go (Fund Transfers)
│
├── pkg/
│   ├── context/context.go (Tenant Context)
│   ├── metrics/metrics.go (Prometheus)
│   ├── vault/vault.go (Secrets)
│   ├── auth/jwt.go (JWT Token Manager)
│   ├── ratelimit/ratelimit.go (Rate Limiter)
│   └── models/models.go (API Models + Database)
│
├── infra/docker/ (Dockerfiles)
├── infra/monitoring/ (Prometheus, Grafana config)
│
├── docker-compose.yml (Local orchestration)
├── Makefile (Commands)
├── README.md (Full documentation)
└── go.mod/go.sum (Dependencies)
```

## Common Commands Cheat Sheet

```bash
# Setup
make local-setup              # Initialize everything
make clean                    # Tear down

# Testing
make test-auth               # Get JWT token
make test-search             # Search account
make test-transfer           # Transfer funds
make test-failure            # Inject failure
make test-rollback           # Demo automatic rollback

# Monitoring
make logs                    # Tail all logs
make logs-edge              # Tail edge logs
make ps                     # Show running containers
make metrics                # Show Prometheus metrics

# URLs
# Edge: http://localhost:8080
# Auth: http://localhost:8081
# Search: http://localhost:8082
# Transfer: http://localhost:8083
# Prometheus: http://localhost:9090
# Grafana: http://localhost:3000
# Kibana: http://localhost:5601
# Jaeger: http://localhost:16686
```

## Stopping & Restarting

```bash
# Graceful shutdown
make swarm-clean

# Restart fresh
make swarm-restart

# deploy service only
make swarm-deploy
```

## Troubleshooting

### "Connection refused" on first run
→ Services take ~10s to start. Run: `docker-compose logs` to monitor.

### "Port already in use"
→ Kill the process: `lsof -ti:8080 | xargs kill -9` (macOS/Linux)

### "Can't connect to Vault"
→ Check: `docker service logs fintech_vault`. Vault may still be initializing.

### Metrics not showing
→ Verify: `http://localhost:9090/targets` - all should show "Up"

### Missing logs in Kibana
→ May take 10s to index. Refresh: `curl -X POST http://localhost:9200/_refresh`

## Next Steps

1. **Explore the code**: Start with `cmd/edge-service/main.go`
2. **Break it**: Try `make test-search` with invalid credentials
3. **Monitor it**: Open Grafana dashboard while running tests
4. **Extend it**: Add new endpoints or services
