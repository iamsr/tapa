#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}TAPA E2E Test Suite (Simplified)${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

# Build TAPA
echo -e "${YELLOW}[1/7] Building TAPA binary...${NC}"
cd "$PROJECT_ROOT"
go build -o "$PROJECT_ROOT/tapa" ./cmd/tapa
echo -e "${GREEN}✓ Built${NC}"
echo ""

# Check Docker containers
echo -e "${YELLOW}[2/7] Checking Docker containers...${NC}"
cd "$SCRIPT_DIR"
if ! docker-compose ps | grep -q "Up"; then
    echo -e "${RED}✗ Containers not running. Start them with: cd tests/e2e && docker-compose up -d${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Containers running${NC}"
echo ""

# Run tests with timeouts
echo -e "${YELLOW}[3/7] PostgreSQL tests...${NC}"
timeout 60 bash "$SCRIPT_DIR/scripts/test_postgres.sh" && echo -e "${GREEN}✓ PostgreSQL PASS${NC}" || echo -e "${RED}✗ PostgreSQL FAIL${NC}"
echo ""

echo -e "${YELLOW}[4/7] MySQL tests...${NC}"
timeout 60 bash "$SCRIPT_DIR/scripts/test_mysql.sh" && echo -e "${GREEN}✓ MySQL PASS${NC}" || echo -e "${RED}✗ MySQL FAIL${NC}"
echo ""

echo -e "${YELLOW}[5/7] Batch tests...${NC}"
timeout 60 bash "$SCRIPT_DIR/scripts/test_batch.sh" && echo -e "${GREEN}✓ Batch PASS${NC}" || echo -e "${RED}✗ Batch FAIL${NC}"
echo ""

echo -e "${YELLOW}[6/7] Time estimation tests...${NC}"
timeout 60 bash "$SCRIPT_DIR/scripts/test_time_estimation.sh" && echo -e "${GREEN}✓ Time estimation PASS${NC}" || echo -e "${RED}✗ Time estimation FAIL${NC}"
echo ""

echo -e "${YELLOW}[7/7] Verbose & color tests...${NC}"
timeout 60 bash "$SCRIPT_DIR/scripts/test_verbose_mode.sh" && echo -e "${GREEN}✓ Verbose PASS${NC}" || echo -e "${RED}✗ Verbose FAIL${NC}"
timeout 60 bash "$SCRIPT_DIR/scripts/test_color_support.sh" && echo -e "${GREEN}✓ Color PASS${NC}" || echo -e "${RED}✗ Color FAIL${NC}"
echo ""

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}All tests completed!${NC}"
echo -e "${GREEN}========================================${NC}"
