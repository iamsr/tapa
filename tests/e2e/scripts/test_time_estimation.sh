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
# Use 127.0.0.1 instead of localhost to avoid IPv6 resolution issues on macOS
PG_URL="postgresql://testuser:testpass@127.0.0.1:5433/testdb"
MYSQL_URL="testuser:testpass@tcp(127.0.0.1:3307)/testdb?tls=false"

echo -e "${BLUE}Testing time estimation accuracy...${NC}"

# Test 1: Time estimates with database connection (100K rows)
echo -e "${YELLOW}  Test 1: PostgreSQL time estimates with database${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --db "$PG_URL" --format json 2>/dev/null)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze${NC}"
    exit 1
fi

# Extract time estimate for first operation
TIME_EST=$(echo "$OUTPUT" | jq -r '.Migrations[0].Operations[0].EstimatedTimeSeconds')
if [ -z "$TIME_EST" ] || [ "$TIME_EST" = "null" ]; then
    echo -e "${RED}    ✗ No time estimate found${NC}"
    exit 1
fi

# Verify time is realistic (between 0.1 and 30 seconds for 100K rows)
if (( $(echo "$TIME_EST < 0.1" | bc -l) )) || (( $(echo "$TIME_EST > 30.0" | bc -l) )); then
    echo -e "${RED}    ✗ Time estimate out of realistic range: ${TIME_EST}s${NC}"
    echo -e "${RED}       Expected: 0.1s - 30.0s for 100K row table${NC}"
    exit 1
fi
echo -e "${GREEN}    ✓ PostgreSQL time estimate realistic: ${TIME_EST}s${NC}"

# Test 2: Time estimates in dry-run mode (no DB)
echo -e "${YELLOW}  Test 2: Dry-run mode time estimates${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --dry-run --format json 2>/dev/null)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze in dry-run${NC}"
    exit 1
fi

TIME_EST_DRYRUN=$(echo "$OUTPUT" | jq -r '.Migrations[0].Operations[0].EstimatedTimeSeconds')
if [ -z "$TIME_EST_DRYRUN" ] || [ "$TIME_EST_DRYRUN" = "null" ]; then
    echo -e "${RED}    ✗ No time estimate in dry-run${NC}"
    exit 1
fi

# Dry-run should produce a positive time estimate
# Simple ADD_COLUMN (no rewrite) is fast (~0.1s), rewrite ops are slower
if (( $(echo "$TIME_EST_DRYRUN <= 0" | bc -l) )); then
    echo -e "${RED}    ✗ Dry-run estimate should be positive: ${TIME_EST_DRYRUN}s${NC}"
    exit 1
fi

# Also verify a rewrite operation has a larger estimate
TIME_EST_REWRITE=$(echo "$OUTPUT" | jq -r '[.Migrations[0].Operations[] | select(.RequiresRewrite == true)] | .[0].EstimatedTimeSeconds')
if [ -n "$TIME_EST_REWRITE" ] && [ "$TIME_EST_REWRITE" != "null" ]; then
    if (( $(echo "$TIME_EST_REWRITE > $TIME_EST_DRYRUN" | bc -l) )); then
        echo -e "${GREEN}    ✓ Dry-run estimates: simple=${TIME_EST_DRYRUN}s, rewrite=${TIME_EST_REWRITE}s${NC}"
    else
        echo -e "${YELLOW}    ⚠ Rewrite estimate (${TIME_EST_REWRITE}s) not larger than simple (${TIME_EST_DRYRUN}s)${NC}"
    fi
else
    echo -e "${GREEN}    ✓ Dry-run estimate: ${TIME_EST_DRYRUN}s${NC}"
fi

# Test 3: Compare PostgreSQL vs MySQL estimates
echo -e "${YELLOW}  Test 3: PostgreSQL vs MySQL estimate comparison${NC}"
PG_OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --db "$PG_URL" --format json 2>/dev/null)
MYSQL_OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/mysql_test_migration.sql" --db "$MYSQL_URL" --format json 2>/dev/null)

PG_TIME=$(echo "$PG_OUTPUT" | jq -r '.Migrations[0].Operations[0].EstimatedTimeSeconds')
MYSQL_TIME=$(echo "$MYSQL_OUTPUT" | jq -r '.Migrations[0].Operations[0].EstimatedTimeSeconds')

# Times should be in reasonable range (within 10x of each other)
# Note: Different estimation algorithms can produce different results with real data
RATIO=$(echo "scale=2; $PG_TIME / $MYSQL_TIME" | bc -l)
if (( $(echo "$RATIO < 0.1" | bc -l) )) || (( $(echo "$RATIO > 10.0" | bc -l) )); then
    echo -e "${RED}    ✗ PostgreSQL and MySQL estimates differ unreasonably${NC}"
    echo -e "${RED}       PostgreSQL: ${PG_TIME}s, MySQL: ${MYSQL_TIME}s, Ratio: ${RATIO}${NC}"
    exit 1
fi
echo -e "${GREEN}    ✓ PostgreSQL (${PG_TIME}s) and MySQL (${MYSQL_TIME}s) estimates in reasonable range (${RATIO}x)${NC}"

# Test 4: Verify batch total time calculation
echo -e "${YELLOW}  Test 4: Batch total time calculation${NC}"
BATCH_OUTPUT=$("$TAPA_BIN" batch "$E2E_DIR/fixtures/postgres_test_migration.sql" --db "$PG_URL" --format json 2>/dev/null)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to run batch command${NC}"
    exit 1
fi

TOTAL_TIME=$(echo "$BATCH_OUTPUT" | jq -r '.strategy.total_time_seconds')
BATCH_COUNT=$(echo "$BATCH_OUTPUT" | jq -r '.strategy.total_batches')

if [ -z "$TOTAL_TIME" ] || [ "$TOTAL_TIME" = "null" ]; then
    echo -e "${RED}    ✗ No total time in batch output${NC}"
    exit 1
fi

# Total time should be positive and reasonable
if (( $(echo "$TOTAL_TIME <= 0" | bc -l) )); then
    echo -e "${RED}    ✗ Invalid total time: ${TOTAL_TIME}s${NC}"
    exit 1
fi

echo -e "${GREEN}    ✓ Batch total time calculated: ${TOTAL_TIME}s across ${BATCH_COUNT} batches${NC}"

echo -e "${GREEN}Time estimation E2E tests completed successfully!${NC}"
