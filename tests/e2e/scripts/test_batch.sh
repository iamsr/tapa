#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
E2E_DIR="$PROJECT_ROOT/tests/e2e"
TAPA_BIN="$PROJECT_ROOT/tapa"

# Database connections
PG_URL="postgresql://testuser:testpass@localhost:5433/testdb"
MYSQL_URL="mysql://testuser:testpass@localhost:3307/testdb"

echo -e "${BLUE}Testing batch command...${NC}"

# Test 1: PostgreSQL batching with JSON output
echo -e "${YELLOW}  Test 1: PostgreSQL batching with JSON output${NC}"
OUTPUT=$("$TAPA_BIN" batch "$E2E_DIR/fixtures/postgres_test_migration.sql" --db "$PG_URL" --format json 2>/dev/null)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed: $OUTPUT${NC}"
    exit 1
fi
echo "$OUTPUT" | jq . > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Invalid JSON output${NC}"
    echo "$OUTPUT"
    exit 1
fi
TOTAL_BATCHES=$(echo "$OUTPUT" | jq '.strategy.total_batches')
if [ -z "$TOTAL_BATCHES" ] || [ "$TOTAL_BATCHES" = "null" ] || [ "$TOTAL_BATCHES" -lt 1 ]; then
    echo -e "${RED}    ✗ Expected at least 1 batch, got $TOTAL_BATCHES${NC}"
    exit 1
fi
echo -e "${GREEN}    ✓ PostgreSQL batching successful (TotalBatches: $TOTAL_BATCHES)${NC}"

# Test 2: MySQL batching with JSON output
echo -e "${YELLOW}  Test 2: MySQL batching with JSON output${NC}"
OUTPUT=$("$TAPA_BIN" batch "$E2E_DIR/fixtures/mysql_test_migration.sql" --db "$MYSQL_URL" --format json 2>/dev/null)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed: $OUTPUT${NC}"
    exit 1
fi
echo "$OUTPUT" | jq . > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Invalid JSON output${NC}"
    echo "$OUTPUT"
    exit 1
fi
TOTAL_BATCHES=$(echo "$OUTPUT" | jq '.strategy.total_batches')
if [ -z "$TOTAL_BATCHES" ] || [ "$TOTAL_BATCHES" = "null" ] || [ "$TOTAL_BATCHES" -lt 1 ]; then
    echo -e "${RED}    ✗ Expected at least 1 batch, got $TOTAL_BATCHES${NC}"
    exit 1
fi
echo -e "${GREEN}    ✓ MySQL batching successful (TotalBatches: $TOTAL_BATCHES)${NC}"

# Test 3: Verify batch structure (check operations exist)
echo -e "${YELLOW}  Test 3: Verify batch structure${NC}"
OUTPUT=$("$TAPA_BIN" batch "$E2E_DIR/fixtures/postgres_test_migration.sql" --db "$PG_URL" --format json 2>/dev/null)
BATCH_OPERATIONS=$(echo "$OUTPUT" | jq '[.strategy.batches[].operations] | add | length')
if [ -z "$BATCH_OPERATIONS" ] || [ "$BATCH_OPERATIONS" = "null" ] || [ "$BATCH_OPERATIONS" -lt 1 ]; then
    echo -e "${RED}    ✗ Expected operations in batches, got $BATCH_OPERATIONS${NC}"
    exit 1
fi
echo -e "${GREEN}    ✓ Batch structure valid (Operations: $BATCH_OPERATIONS)${NC}"

# Test 4: Table format output works
echo -e "${YELLOW}  Test 4: Table format output${NC}"
echo -e "${BLUE}  Running: tapa batch postgres_test_migration.sql --db <postgres> --format table${NC}"
echo ""
"$TAPA_BIN" batch "$E2E_DIR/fixtures/postgres_test_migration.sql" --db "$PG_URL" --format table 2>&1
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Table format output failed${NC}"
    exit 1
fi
echo ""
echo -e "${GREEN}    ✓ Table format output successful${NC}"

echo -e "${GREEN}Batch command E2E tests completed successfully!${NC}"
