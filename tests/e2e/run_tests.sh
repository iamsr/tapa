#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
E2E_DIR="$PROJECT_ROOT/tests/e2e"

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}TAPA End-to-End Test Suite (10 sections)${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

# Build TAPA binary
echo -e "${YELLOW}[1/10] Building TAPA binary...${NC}"
cd "$PROJECT_ROOT"
go build -o "$PROJECT_ROOT/tapa" ./cmd/tapa
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Failed to build TAPA${NC}"
    exit 1
fi
echo -e "${GREEN}✓ TAPA binary built successfully${NC}"
echo ""

# Start Docker containers
echo -e "${YELLOW}[2/10] Starting Docker containers...${NC}"
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
echo -e "${YELLOW}[3/10] Waiting for databases to be ready...${NC}"
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

# Wait for seed data to be loaded (check row counts)
echo -n "  Waiting for seed data"
for i in {1..60}; do
    PG_COUNT=$(docker exec tapa-e2e-postgres psql -U testuser -d testdb -t -c "SELECT COUNT(*) FROM users;" 2>/dev/null | tr -d ' ' || echo "0")
    MYSQL_COUNT=$(docker exec tapa-e2e-mysql mysql -u testuser -ptestpass -D testdb -N -e "SELECT COUNT(*) FROM users;" 2>/dev/null || echo "0")
    
    if [ "$PG_COUNT" = "100000" ] && [ "$MYSQL_COUNT" = "100000" ]; then
        echo -e " ${GREEN}✓${NC}"
        break
    fi
    echo -n "."
    sleep 2
    if [ $i -eq 60 ]; then
        echo -e " ${RED}✗ Timeout waiting for seed data${NC}"
        echo "PostgreSQL rows: $PG_COUNT, MySQL rows: $MYSQL_COUNT"
        docker-compose down -v
        exit 1
    fi
done
echo ""

# Run PostgreSQL E2E test
echo -e "${YELLOW}[4/10] Running PostgreSQL E2E test...${NC}"
bash "$E2E_DIR/scripts/test_postgres.sh"
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ PostgreSQL E2E test failed${NC}"
    docker-compose down -v
    exit 1
fi
echo -e "${GREEN}✓ PostgreSQL E2E test passed${NC}"
echo ""

# Run MySQL E2E test
echo -e "${YELLOW}[5/10] Running MySQL E2E test...${NC}"
bash "$E2E_DIR/scripts/test_mysql.sh"
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ MySQL E2E test failed${NC}"
    docker-compose down -v
    exit 1
fi
echo -e "${GREEN}✓ MySQL E2E test passed${NC}"
echo ""

# Run Batch command E2E test
echo -e "${YELLOW}[6/10] Running Batch command E2E test...${NC}"
bash "$E2E_DIR/scripts/test_batch.sh"
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Batch command E2E test failed${NC}"
    docker-compose down -v
    exit 1
fi
echo -e "${GREEN}✓ Batch command E2E test passed${NC}"
echo ""

# Run Time estimation E2E test
echo -e "${YELLOW}[7/10] Running Time estimation E2E test...${NC}"
bash "$E2E_DIR/scripts/test_time_estimation.sh"
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Time estimation E2E test failed${NC}"
    docker-compose down -v
    exit 1
fi
echo -e "${GREEN}✓ Time estimation E2E test passed${NC}"
echo ""

# Run Progress output E2E test
echo -e "${YELLOW}[8/10] Running Progress output E2E test...${NC}"
bash "$E2E_DIR/scripts/test_verbose_mode.sh"
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Progress output E2E test failed${NC}"
    docker-compose down -v
    exit 1
fi
echo -e "${GREEN}✓ Progress output E2E test passed${NC}"
echo ""

# Run Color support E2E test
echo -e "${YELLOW}[9/10] Running Color support E2E test...${NC}"
bash "$E2E_DIR/scripts/test_color_support.sh"
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Color support E2E test failed${NC}"
    docker-compose down -v
    exit 1
fi
echo -e "${GREEN}✓ Color support E2E test passed${NC}"
echo ""

# Cleanup
echo -e "${YELLOW}[10/10] Cleaning up...${NC}"
docker-compose down -v > /dev/null 2>&1
echo -e "${GREEN}✓ Cleanup complete${NC}"
echo ""

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}All E2E tests passed! ✓${NC}"
echo -e "${GREEN}========================================${NC}"
