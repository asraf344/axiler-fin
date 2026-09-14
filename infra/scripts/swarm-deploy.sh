#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

STACK_NAME="fintech"
COMPOSE_FILE="docker-swarm-stack.yml"
ENV_FILE="${1:-.env.swarm}"

echo -e "${YELLOW}=== Fintech Platform - Swarm Deployment ===${NC}"
echo ""

# Check if Swarm is active
if ! docker info | grep -q "Swarm: active"; then
    echo -e "${RED}Error: Docker Swarm is not active${NC}"
    echo "Run: make swarm-init"
    exit 1
fi

# Check if compose file exists
if [ ! -f "$COMPOSE_FILE" ]; then
    echo -e "${RED}Error: $COMPOSE_FILE not found${NC}"
    exit 1
fi

# Check if env file exists
if [ ! -f "$ENV_FILE" ]; then
    echo -e "${YELLOW}Warning: $ENV_FILE not found, using defaults${NC}"
    echo "Copy .env.swarm.example to $ENV_FILE and update values"
fi

echo "Loading environment from: $ENV_FILE"
if [ -f "$ENV_FILE" ]; then
    set -a
    source "$ENV_FILE"
    set +a
fi

# Default values if not set
REGISTRY="${REGISTRY:-fintech}"
EDGE_VERSION="${EDGE_VERSION:-latest}"
AUTH_VERSION="${AUTH_VERSION:-latest}"
SEARCH_VERSION="${SEARCH_VERSION:-latest}"
TRANSFER_VERSION="${TRANSFER_VERSION:-latest}"

echo "Configuration:"
echo "  Stack Name: $STACK_NAME"
echo "  Registry: $REGISTRY"
echo "  Edge Version: $EDGE_VERSION"
echo "  Auth Version: $AUTH_VERSION"
echo "  Search Version: $SEARCH_VERSION"
echo "  Transfer Version: $TRANSFER_VERSION"
echo ""

# Validate that required secrets exist
echo "Checking required secrets..."
if ! docker secret ls | grep -q vault_token; then
    echo -e "${RED}Error: vault_token secret not found${NC}"
    echo "Run: make swarm-init"
    exit 1
fi
echo -e "${GREEN}✓ vault_token secret exists${NC}"

if ! docker secret ls | grep -q grafana_admin_password; then
    echo -e "${RED}Error: grafana_admin_password secret not found${NC}"
    echo "Run: make swarm-init"
    exit 1
fi
echo -e "${GREEN}✓ grafana_admin_password secret exists${NC}"

echo ""
echo "Validating compose file..."
docker-compose -f "$COMPOSE_FILE" config > /dev/null 2>&1
echo -e "${GREEN}✓ Compose file is valid${NC}"

echo ""
echo -e "${BLUE}Deploying stack: $STACK_NAME${NC}"

# Deploy or update the stack
docker stack deploy \
    -c "$COMPOSE_FILE" \
    --with-registry-auth \
    "$STACK_NAME"

echo -e "${GREEN}✓ Stack deployment initiated${NC}"

echo ""
echo "Waiting for services to stabilize (30 seconds)..."
sleep 30

echo ""
echo "=== Service Status ==="
docker stack services "$STACK_NAME"

echo ""
echo "=== Task Status (first 20) ==="
docker stack ps "$STACK_NAME" | head -25

echo ""
echo -e "${GREEN}=== Deployment Complete ===${NC}"
echo ""
echo "Monitoring commands:"
echo "  docker stack services $STACK_NAME                    # Service status"
echo "  docker stack ps $STACK_NAME                          # Task status"
echo "  docker service logs -f $STACK_NAME-edge              # Edge logs"
echo "  docker service logs -f $STACK_NAME-transfer          # Transfer logs"
echo ""
echo "Access URLs:"
echo "  Edge:        http://localhost:8080/health"
echo "  Prometheus:  http://localhost:9090"
echo "  Grafana:     http://localhost:3000"
echo "  Kibana:      http://localhost:5601"
echo "  Jaeger:      http://localhost:16686"
echo ""
echo "To scale services:"
echo "  docker service scale $STACK_NAME-edge=5"
echo "  docker service scale $STACK_NAME-transfer=5"
echo ""
echo "To rollback to previous version:"
echo "  docker service rollback $STACK_NAME-transfer"
echo ""
