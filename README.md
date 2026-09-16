# Axiler-Fin - Secure Multi-Tenant Architecture

A production-inspired demonstration of a secure, multi-tenant financial services platform with complete observability, security gates, and deployment automation.

## Quick Start

### Prerequisites
- Docker & Docker Swarm (for mac need to install docker desktop)
- `curl` and `jq` (for testing)

### One-Command Deployment

```bash
make  build-images swarm-init  swarm-deploy
```

This:
1. Builds all Docker images
2. Starts services: Edge, Auth, Search, Transfer, Vault, Prometheus, Elasticsearch, Kibana, Grafana, Jaeger
3. Seeds test data (3 tenants: alpha, beta, gamma)
4. Outputs service URLs

**Result:** Platform fully operational in ~30 seconds

## Architecture

```
External Clients (alpha, beta, gamma)
        ↓
Security Service Edge (SSE)
  ├─ mTLS validation
  ├─ Rate limiting per tenant
  ├─ Secret scanning
  ├─ Abuse detection
  └─ Audit logging
        ↓
├─ Auth Service (JWT issuance)
├─ Search Service (account lookup)
└─ Transfer Service (fund transfers)
        ↓
├─ Vault (secrets management)
├─ In-memory Database (synthetic data)
└─ Observability Stack
    ├─ Prometheus (metrics)
    ├─ ELK (logs)
    ├─ Jaeger (traces)
    └─ Grafana (dashboard)
```

## Monitoring URLs

Open in browser:
- **Grafana Dashboard**: http://localhost:3000 (admin/admin)
  - Request rates by tenant
  - Error rates and latency
  - Service health status
  - Release version tracking

- **Prometheus**: http://localhost:9090
  - Metrics browser
  - Query language for debugging

- **Kibana**: http://localhost:5601
  - Structured logs
  - Tenant activity timeline
  - Security events

- **Jaeger**: http://localhost:16686
  - Distributed traces
  - Request flow visualization

- **Vault**: http://localhost:8200
  - Token: `dev-token`
  - Secrets UI

## API Testing

### 1. Authentication (get JWT token)

```bash
make swarm-test-auth
```

Or manually:
```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"api_key":"alpha_key_123","secret":"alpha_secret_456"}'
```

Response includes JWT token valid for 24 hours.

**Available tenants:**
- `alpha`: `alpha_key_123` / `alpha_secret_456`
- `beta`: `beta_key_789` / `beta_secret_012`
- `gamma`: `gamma_key_345` / `gamma_secret_678`

### 2. Search (account lookup)

```bash
make swarm-test-search
```

This:
1. Authenticates as tenant `alpha`
2. Searches for account `acc_alpha_001`
3. Shows account balance and status

**Tenant Isolation Check:** If you try to search for `acc_beta_001` using alpha's token, you'll get a `403 Forbidden` error. The service verifies that the account belongs to the requesting tenant before returning data.

### 3. Transfer (fund movement)

```bash
make swarm-test-transfer
```

Transfers $1500 from `acc_alpha_001` to `acc_alpha_002`.

### 4. Rate Limiting

```bash
# Tenant alpha has 1000 req/min limit
# Tenant beta has 500 req/min limit
# Tenant gamma has 250 req/min limit

curl http://localhost:8080/admin/ratelimit-status/alpha | jq .
```

After 1000 requests per minute, subsequent requests from alpha get `429 Too Many Requests`.

## Security Features Demonstrated

### 1. Multi-Tenant Isolation

Each request includes tenant context via JWT. Services enforce:
- **Database isolation**: Queries include `WHERE tenant_id = $1`
- **Network isolation**: Overlay networks per tenant (in Swarm)
- **Request-level validation**: Cross-tenant access is logged as security violation

Test cross-tenant access:
```bash
# Alpha's token + beta's account = forbidden
curl -X POST http://localhost:8080/api/v1/search \
  -H "Authorization: Bearer {alpha_token}" \
  -d '{"account_id":"acc_beta_001"}'
# Returns 403 Forbidden, logged as SECURITY VIOLATION
```

### 2. Edge Security Controls

The SSE (Security Service Edge) at port 8080 implements:

- **Rate Limiting**: Token bucket per tenant
- **Secret Scanning**: Detects potential secrets in request bodies
- **Abuse Detection**: Pattern matching for suspicious behavior
- **Request Logging**: All requests logged with trace ID + tenant context
- **Response Headers**: Security headers added (CSP, HSTS, etc.)

### 3. Structured Logging

All logs include:
- `tenant_id`: Which tenant's request is it?
- `trace_id`: Correlates logs across service calls
- `request_id`: Unique per request
- `user_id`: Which user made the request
- `release_version`: Which service version handled it

Example query in Kibana:
```
trace_id: trace_12345678
```
Shows all logs from a single request across all services.

### 4. Metrics & Alerting

Prometheus tracks:
- `http_requests_total` by tenant, endpoint, status
- `http_request_duration_seconds` with percentiles
- `edge_ratelimit_hits_total` by tenant
- `edge_auth_failures_total` by reason
- `edge_suspicious_activity_total` by reason
- `service_health_status` per service

Alert examples:
- Error rate > 5% for 2 minutes → alert
- Service health check failing > 30s → automatic rollback
- Suspicious activity spike → alert

## Deployment & Rollback

### Simulating Unhealthy Release

```bash
# Inject failure into transfer service
make test-failure

# Health checks will fail within 10s
docker service logs fintech_transfer

# In Swarm, automatic rollback triggers after 3 consecutive health check failures
# View rollback status:
docker service ps fintech-fintech_transfer
```

### Manual Rollback

```bash
# Rollback to previous working version
docker service rollback fintech_transfer

# Or manually update to specific digest
docker service update --image registry.example.com/transfer:sha256:abc123... transfer
```

## Project Structure

```
fintech-platform/
├── cmd/                          # Service entry points
│   ├── edge-service/main.go      # API Gateway (mTLS, rate limit, secret scan)
│   ├── auth-service/main.go      # JWT token issuer
│   ├── search-service/main.go    # Account lookup (tenant-isolated)
│   └── transfer-service/main.go  # Fund transfers (with health check simulation)
│
├── pkg/                          # Shared libraries
│   ├── context/context.go        # Tenant context + trace ID
│   ├── metrics/metrics.go        # Prometheus instrumentation
│   ├── vault/vault.go            # Vault client (AppRole auth)
│   ├── auth/jwt.go               # JWT creation & validation
│   ├── ratelimit/ratelimit.go    # Token bucket rate limiter
│   └── models/models.go          # API models + synthetic database
│
├── infra/
│   ├── docker/                   # Dockerfiles (multi-stage builds)
│   │   ├── Dockerfile.edge
│   │   ├── Dockerfile.auth
│   │   ├── Dockerfile.search
│   │   └── Dockerfile.transfer
│   │
│   └── monitoring/
│       ├── prometheus.yml        # Prometheus scrape config
│       ├── alert_rules.yml       # Alerting rules
│       └── grafana-*.yml         # Grafana datasources & dashboards
│
├── docker-swarm-stack.yml            # Local dev orchestration
├── Makefile                       # Common commands
├── go.mod / go.sum              # Dependencies
└── README.md                      # This file
```

## Key Go Packages Used

- **chi/v5**: Lightweight HTTP router
- **prometheus/client_golang**: Metrics instrumentation
- **golang-jwt/jwt/v5**: JWT token handling
- **hashicorp/vault/api**: Vault client
- **sirupsen/logrus**: Structured logging

## What Each Service Does

### Edge Service (Port 8080)

- **Role**: API Gateway (Security Service Edge)
- **Controls**:
  - mTLS client certificate validation
  - Per-tenant rate limiting (token bucket)
  - Secret pattern scanning in request bodies
  - Abuse detection
  - Request/response logging
  - Security headers (CSP, HSTS, X-Frame-Options)
- **Metrics**: `edge_auth_failures_total`, `edge_ratelimit_hits_total`, `edge_suspicious_activity_total`

### Auth Service (Port 8081)

- **Role**: Token Issuer
- **Endpoints**:
  - `POST /auth/login`: Exchange API key + secret for JWT
  - `GET /health`: Service health
- **Features**:
  - RSA-256 signature
  - 24-hour token expiry
  - Tenant ID embedded in token claims

### Search Service (Port 8082)

- **Role**: Account Lookup
- **Endpoints**:
  - `POST /api/v1/search`: Lookup account by ID (tenant-isolated)
  - `GET /health`: Service health
- **Security**:
  - Requires valid JWT
  - Verifies account belongs to requesting tenant
  - Logs cross-tenant access attempts as security violations

### Transfer Service (Port 8083)

- **Role**: Fund Transfer Processing
- **Endpoints**:
  - `POST /api/v1/transfer`: Initiate transfer
  - `GET /health`: Service health (can be made unhealthy for testing)
  - `POST /admin/simulate-failure`: Toggle failure mode
- **Features**:
  - Balance validation
  - Tenant isolation checks
  - Ledger entries
  - Configurable failure mode for testing rollback

## Testing Scenarios

### Scenario 1: Tenant Isolation

```bash
# Get token for alpha
TOKEN_ALPHA=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"api_key":"alpha_key_123","secret":"alpha_secret_456"}' | jq -r .token)

# Try to access beta's account
curl -X POST http://localhost:8080/api/v1/search \
  -H "Authorization: Bearer $TOKEN_ALPHA" \
  -d '{"account_id":"acc_beta_001"}'

# Result: 403 Forbidden
# Logs show: "Attempted to access account from different tenant - SECURITY VIOLATION"
```

### Scenario 2: Rate Limiting

```bash
# This will exceed gamma's limit (250 req/min) if run repeatedly
for i in {1..300}; do
  curl -s http://localhost:8080/admin/ratelimit-status/gamma > /dev/null
done

# After ~250 requests, you'll get 429 Too Many Requests
```

### Scenario 3: Service Failure & Rollback

```bash
# 1. Inject failure
make test-failure

# 2. Observe: transfer service health check fails
docker logs fintech-transfer | grep "Health check failed"

# 3. In production Swarm:
#    - After 3 consecutive failures, automatic rollback triggers
#    - Previous image digest is restored
#    - Service becomes healthy again

# 4. Disable failure
make test-failure  # Toggle off
```

### Scenario 4: Observability

```bash
# Check all active metrics
curl -s http://localhost:9090/api/v1/query?query=up | jq '.data.result'

# Find error rate by tenant in the last hour
# In Prometheus UI:
#   rate(http_requests_total{status=~"5.."}[1h]) by (tenant)

# Find all logs for trace_id
# In Kibana:
#   trace_id: "trace_123456"
```

## Common Issues & Troubleshooting

### Services not starting

```bash
# Check Docker Compose logs
docker-compose logs

# Rebuild images
docker-compose build --no-cache

# Restart
docker-compose down -v && docker-compose up -d
```

### Cannot connect to Vault

```bash
# Vault may be taking time to start
docker logs fintech-vault

# Wait for "Vault is unsealed and ready to accept requests"
```

### Health checks failing

```bash
# Check if services are actually running
docker-compose ps

# Check individual service health
curl http://localhost:8082/health  # search service
curl http://localhost:8083/health  # transfer service
```

### Metrics not appearing in Prometheus

```bash
# Verify Prometheus is scraping targets
# Visit: http://localhost:9090/targets
# All targets should show "Up"

# If "Down", check service logs and health endpoints
```

# Improvements to be done

- Backends configuration at the edge should be moved to a config rather than at go code
- More Efficien Abuse detection
- Account transfer only shows successful message but it doesn't reflect at search/get, as transfer and search are two different services and DB is in memory.
- Didn't get time for Writing unit tests
- Use keyless approach for cosign, not that much convenient at CI/CD as it waits for github OIDC login

## Security Notes

### What's Production-Ready
- JWT token validation & signature verification
- Tenant isolation enforcement (request-level)
- Structured logging with trace IDs
- Prometheus metrics instrumentation
- Health checks with automatic rollback

### What's Simplified for Demo
- RSA keys are hardcoded (use Vault in production)
- Tenant api keys are hardcoded shoul
- Database credentials in environment (use Vault + mTLS in production)
- In-memory database (use PostgreSQL with row-level security)
- mTLS is not enforced locally (enable in production)
- No encryption at rest (add for production)

### For Production Deployment

1. **Secrets Management**:
   - Generate strong RSA keys
   - Store in Vault with AppRole auth
   - Rotate credentials regularly

2. **Network**:
   - Use TLS for all inter-service communication
   - Enable mTLS with client cert validation
   - Implement service-to-service authorization

3. **Database**:
   - Use PostgreSQL with row-level security
   - Dynamic credentials from Vault
   - Connection pooling + circuit breakers

4. **Observability**:
   - Centralize logs to ELK
   - Set up alerting rules
   - Implement SLOs/SLIs

5. **Deployment**:
   - Sign container images with Cosign
   - Use digest-based references (not tags)
   - Generate SBOMs with Syft
   - Use container scanning (Trivy)

## References

- [Implementation Summary](./docs/IMPLEMENTATION_SUMMARY.md) 
- [Quick Start Guide](./docs/QUICKSTART.md)
- [Vault Documentation](https://www.vaultproject.io/)
- [Prometheus Operator](https://prometheus-operator.dev/)
- [Docker Swarm Secrets](https://docs.docker.com/engine/swarm/secrets/)
