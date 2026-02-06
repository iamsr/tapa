#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
E2E_DIR="$PROJECT_ROOT/e2e"

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}TAPA End-to-End Test Suite${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

# Build TAPA binary
echo -e "${YELLOW}[1/7] Building TAPA binary...${NC}"
cd "$PROJECT_ROOT"
go build -o "$PROJECT_ROOT/tapa" ./cmd/tapa
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Failed to build TAPA${NC}"
    exit 1
fi
echo -e "${GREEN}✓ TAPA binary built successfully${NC}"
echo ""

# Start Docker containers
echo -e "${YELLOW}[2/7] Starting Docker containers...${NC}"
cd "$E2E_DIR"
docker-compose down -v > /dev/null 2>&1 || true
docker-compose up -d
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Failed to start Docker containers${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Docker containers started${NC}"
echo ""

# Wait for databases to be ready
echo -e "${YELLOW}[3/7] Waiting for databases to be ready...${NC}"
echo -n "  Waiting for PostgreSQL"
for i in {1..30}; do
    if docker exec tapa-e2e-postgres pg_isready -U testuser -d testdb > /dev/null 2>&1; then
        echo -e " ${GREEN}✓${NC}"
        break
    fi
    echo -n "."
    sleep 1
    if [ $i -eq 30 ]; then
        echo -e " ${RED}✗ Timeout${NC}"
        docker-compose logs postgres
        docker-compose down -v
        exit 1
    fi
done

echo -n "  Waiting for MySQL"
for i in {1..30}; do
    if docker exec tapa-e2e-mysql mysqladmin ping -h localhost -u testuser -ptestpass --silent > /dev/null 2>&1; then
        echo -e " ${GREEN}✓${NC}"
        break
    fi
    echo -n "."
    sleep 1
    if [ $i -eq 30 ]; then
        echo -e " ${RED}✗ Timeout${NC}"
        docker-compose logs mysql
        docker-compose down -v
        exit 1
    fi
done
echo ""

# Run PostgreSQL E2E test
echo -e "${YELLOW}[4/7] Running PostgreSQL E2E test...${NC}"
bash "$E2E_DIR/scripts/test_postgres.sh"
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ PostgreSQL E2E test failed${NC}"
    docker-compose down -v
    exit 1
fi
echo -e "${GREEN}✓ PostgreSQL E2E test passed${NC}"
echo ""

# Run MySQL E2E test
echo -e "${YELLOW}[5/7] Running MySQL E2E test...${NC}"
bash "$E2E_DIR/scripts/test_mysql.sh"
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ MySQL E2E test failed${NC}"
    docker-compose down -v
    exit 1
fi
echo -e "${GREEN}✓ MySQL E2E test passed${NC}"
echo ""

# Run Batch command E2E test
echo -e "${YELLOW}[6/7] Running Batch command E2E test...${NC}"
bash "$E2E_DIR/scripts/test_batch.sh"
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Batch command E2E test failed${NC}"
    docker-compose down -v
    exit 1
fi
echo -e "${GREEN}✓ Batch command E2E test passed${NC}"
echo ""

# Cleanup
echo -e "${YELLOW}[7/7] Cleaning up...${NC}"
docker-compose down -v > /dev/null 2>&1
echo -e "${GREEN}✓ Cleanup complete${NC}"
echo ""

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}All E2E tests passed! ✓${NC}"
echo -e "${GREEN}========================================${NC}"
