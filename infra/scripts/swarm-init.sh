#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== Fintech Platform - Swarm Initialization ===${NC}"
echo ""

# Check if Docker is running
if ! docker info &> /dev/null; then
    echo -e "${RED}Error: Docker daemon is not running${NC}"
    exit 1
fi

# Check if already in Swarm mode
if docker info | grep -q "Swarm: active"; then
    echo -e "${YELLOW}Swarm is already active${NC}"
    SWARM_STATUS="active"
else
    echo "Initializing Docker Swarm..."
    docker swarm init
    echo -e "${GREEN}✓ Swarm initialized${NC}"
    SWARM_STATUS="initialized"
fi

echo ""
echo "Getting Swarm info..."
SWARM_NODE=$(docker info | grep "NodeID" | awk '{print $2}')
SWARM_LEADER=$(docker node inspect self | jq -r '.[0].ManagerStatus.Leader')
echo "Node ID: $SWARM_NODE"
echo "Leader: $SWARM_LEADER"

echo ""
echo "Labeling nodes for service placement..."

# Get all nodes
NODES=$(docker node ls -q)

# Label manager nodes
for node in $NODES; do
    ROLE=$(docker node inspect "$node" | jq -r '.[0].Spec.Role')
    
    if [ "$ROLE" = "manager" ]; then
        echo "Labeling manager node: $node"
        docker node update --label-add service=edge "$node" || true
        docker node update --label-add service=auth "$node" || true
        docker node update --label-add service=search "$node" || true
        docker node update --label-add service=transfer "$node" || true
        docker node update --label-add stability=high "$node" || true
    fi
done

echo -e "${GREEN}✓ Nodes labeled${NC}"

echo ""
echo "Creating Swarm secrets..."

# Create vault token secret if it doesn't exist
if docker secret ls | grep -q vault_token; then
    echo "Secret 'vault_token' already exists, skipping..."
else
    echo "dev-token" | docker secret create vault_token - || true
    echo -e "${GREEN}✓ Created vault_token secret${NC}"
fi

# Create grafana admin password secret if it doesn't exist
if docker secret ls | grep -q grafana_admin_password; then
    echo "Secret 'grafana_admin_password' already exists, skipping..."
else
    echo "admin" | docker secret create grafana_admin_password - || true
    echo -e "${GREEN}✓ Created grafana_admin_password secret${NC}"
fi

echo ""
echo "Creating Swarm configs..."

# Create prometheus config if it doesn't exist
if docker config ls | grep -q prometheus_config; then
    echo "Config 'prometheus_config' already exists, removing old version..."
    docker config rm prometheus_config || true
fi

if [ -f "./infra/monitoring/prometheus.yml" ]; then
    docker config create prometheus_config ./infra/monitoring/prometheus.yml
    echo -e "${GREEN}✓ Created prometheus_config${NC}"
else
    echo -e "${RED}Warning: prometheus.yml not found${NC}"
fi

# Create prometheus alerts config if it doesn't exist
if docker config ls | grep -q prometheus_alerts; then
    echo "Config 'prometheus_alerts' already exists, removing old version..."
    docker config rm prometheus_alerts || true
fi

if [ -f "./infra/monitoring/prometheus-swarm-alerts.yml" ]; then
    docker config create prometheus_alerts ./infra/monitoring/prometheus-swarm-alerts.yml
    echo -e "${GREEN}✓ Created prometheus_alerts${NC}"
else
    echo -e "${YELLOW}Warning: prometheus-swarm-alerts.yml not found (creating default)${NC}"
    # Create default alerts file
    mkdir -p ./infra/monitoring
    cat > ./infra/monitoring/prometheus-swarm-alerts.yml << 'EOF'
groups:
  - name: fintech_platform
    interval: 30s
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected (>5%)"
          description: "Service {{ $labels.service }} has error rate {{ $value }}"

      - alert: HealthCheckFailing
        expr: service_health_status == 0
        for: 30s
        labels:
          severity: critical
        annotations:
          summary: "Service health check failing"
          description: "Service {{ $labels.service }} is unhealthy"

      - alert: RateLimitExceeded
        expr: increase(edge_ratelimit_hits_total[5m]) > 10
        labels:
          severity: warning
        annotations:
          summary: "Rate limit exceeded for tenant {{ $labels.tenant }}"

      - alert: SuspiciousActivity
        expr: increase(edge_suspicious_activity_total[5m]) > 5
        labels:
          severity: warning
        annotations:
          summary: "Suspicious activity detected: {{ $labels.reason }}"

      - alert: ServiceDown
        expr: up == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Service {{ $labels.service }} is down"
EOF
    docker config create prometheus_alerts ./infra/monitoring/prometheus-swarm-alerts.yml
    echo -e "${GREEN}✓ Created default prometheus_alerts${NC}"
fi

# Create grafana datasources config if it doesn't exist
if docker config ls | grep -q grafana_datasources; then
    echo "Config 'grafana_datasources' already exists, removing old version..."
    docker config rm grafana_datasources || true
fi

if [ -f "./infra/monitoring/grafana-datasources.yml" ]; then
    docker config create grafana_datasources ./infra/monitoring/grafana-datasources.yml
    echo -e "${GREEN}✓ Created grafana_datasources${NC}"
else
    echo -e "${YELLOW}Warning: grafana-datasources.yml not found (creating default)${NC}"
    mkdir -p ./infra/monitoring
    cat > ./infra/monitoring/grafana-datasources.yml << 'EOF'
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: true
EOF
    docker config create grafana_datasources ./infra/monitoring/grafana-datasources.yml
    echo -e "${GREEN}✓ Created default grafana_datasources${NC}"
fi

echo ""
echo "Creating network overlay if needed..."
docker network ls | grep -q fintech-overlay || docker network create -d overlay --opt com.docker.network.driver.overlay.vxlan_list=4097 fintech-overlay || true

echo ""
echo -e "${GREEN}=== Swarm Initialization Complete ===${NC}"
echo ""
echo "Next steps:"
echo "1. Edit .env.swarm with your registry and image digests"
echo "2. Run: make swarm-deploy"
echo "3. Monitor: docker stack services fintech"
echo ""
echo "To add worker nodes, run on worker:"
echo "  docker swarm join --token <token> <manager-ip>:2377"
echo ""
echo "To get the token:"
echo "  docker swarm join-token worker"
echo ""
