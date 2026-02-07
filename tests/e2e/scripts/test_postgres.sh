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

# PostgreSQL connection
# Use 127.0.0.1 instead of localhost to avoid IPv6 resolution issues on macOS
PG_URL="postgresql://testuser:testpass@127.0.0.1:5433/testdb"

echo -e "${BLUE}Testing PostgreSQL integration...${NC}"

# Test 1: Basic analysis without database connection (dry-run)
echo -e "${YELLOW}  Test 1: Dry-run analysis${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --dry-run --format json 2>/dev/null)
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
MIGRATION_COUNT=$(echo "$OUTPUT" | jq '.Migrations | length')
if [ "$MIGRATION_COUNT" -lt 1 ]; then
    echo -e "${RED}    ✗ No migrations found in output${NC}"
    exit 1
fi
echo -e "${GREEN}    ✓ Dry-run analysis successful${NC}"

# Test 2: Analysis with database connection
echo -e "${YELLOW}  Test 2: Analysis with database connection${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --db "$PG_URL" --format json 2>/dev/null)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed: $OUTPUT${NC}"
    exit 1
fi
echo "$OUTPUT" | jq . > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Invalid JSON output${NC}"
    exit 1
fi
OPERATION_COUNT=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[]] | length')
if [ "$OPERATION_COUNT" -lt 3 ]; then
    echo -e "${RED}    ✗ Expected at least 3 operations, got $OPERATION_COUNT${NC}"
    exit 1
fi
# Verify real database connection by checking row_count is NOT the dry-run default (1M)
# Real DB has ~100K rows, dry-run defaults to 1M
ROW_COUNT=$(echo "$OUTPUT" | jq '.Migrations[0].Operations[0].row_count')
if [ "$ROW_COUNT" = "1000000" ] || [ "$ROW_COUNT" = "null" ]; then
    echo -e "${RED}    ✗ Database connection failed - got dry-run defaults (row_count: $ROW_COUNT)${NC}"
    exit 1
fi
echo -e "${GREEN}    ✓ Database connection analysis successful (row_count: $ROW_COUNT)${NC}"

# Test 3: Comprehensive analysis (Phase 2 features)
echo -e "${YELLOW}  Test 3: Comprehensive analysis (Phase 2 features)${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --db "$PG_URL" --comprehensive --format json 2>/dev/null)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed: $OUTPUT${NC}"
    exit 1
fi

# Check for Phase 2 features in output
HAS_TIME_BREAKDOWN=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[] | select(.TimeBreakdown != null)] | length')
if [ "$HAS_TIME_BREAKDOWN" -lt 1 ]; then
    echo -e "${YELLOW}    ⚠ Note: No time breakdown detected${NC}"
else
    echo -e "${GREEN}    ✓ Time breakdown present in $HAS_TIME_BREAKDOWN operations${NC}"
fi

# Check for alternatives
HAS_ALTERNATIVES=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[] | select(.Alternatives != null and (.Alternatives | length) > 0)] | length')
if [ "$HAS_ALTERNATIVES" -lt 1 ]; then
    echo -e "${YELLOW}    ⚠ Note: No alternatives generated (may be low risk operations)${NC}"
else
    echo -e "${GREEN}    ✓ Alternatives generated for $HAS_ALTERNATIVES operations${NC}"
fi
echo -e "${GREEN}    ✓ Comprehensive analysis successful${NC}"

# Test 4: Check for specific operation types
echo -e "${YELLOW}  Test 4: Verify operation detection${NC}"
ADD_COLUMN_COUNT=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[] | select(.Type == "ADD_COLUMN")] | length')
CREATE_INDEX_COUNT=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[] | select(.Type == "CREATE_INDEX")] | length')

if [ "$ADD_COLUMN_COUNT" -lt 1 ]; then
    echo -e "${RED}    ✗ Expected ADD_COLUMN operations${NC}"
    exit 1
fi
if [ "$CREATE_INDEX_COUNT" -lt 1 ]; then
    echo -e "${RED}    ✗ Expected CREATE_INDEX operations${NC}"
    exit 1
fi
echo -e "${GREEN}    ✓ Operations detected correctly: ADD_COLUMN($ADD_COLUMN_COUNT), CREATE_INDEX($CREATE_INDEX_COUNT)${NC}"

# Test 5: Check for risk scoring
echo -e "${YELLOW}  Test 5: Verify risk scoring${NC}"
MEDIUM_RISK=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[] | select(.RiskScore >= 40 and .RiskScore < 70)] | length')
HIGH_RISK=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[] | select(.RiskScore >= 70)] | length')

if [ "$MEDIUM_RISK" -lt 1 ] && [ "$HIGH_RISK" -lt 1 ]; then
    echo -e "${RED}    ✗ Expected some operations with risk scores${NC}"
    exit 1
fi
echo -e "${GREEN}    ✓ Risk scoring working: MEDIUM($MEDIUM_RISK), HIGH($HIGH_RISK)${NC}"

# Test 6: Verify recommendations
echo -e "${YELLOW}  Test 6: Verify recommendations${NC}"
HAS_RECOMMENDATIONS=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[] | select(.Recommendations != null and (.Recommendations | length) > 0)] | length')
if [ "$HAS_RECOMMENDATIONS" -lt 1 ]; then
    echo -e "${RED}    ✗ Expected recommendations for risky operations${NC}"
    exit 1
fi
echo -e "${GREEN}    ✓ Recommendations provided for $HAS_RECOMMENDATIONS operations${NC}"

# Test 7: Show actual program output (table format)
echo -e "${YELLOW}  Test 7: Sample output (table format)${NC}"
echo -e "${BLUE}  Running: tapa analyze postgres_test_migration.sql --db <postgres> --format table${NC}"
echo ""
"$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --db "$PG_URL" --format table 2>&1 || true
echo ""

echo -e "${GREEN}PostgreSQL E2E tests completed successfully!${NC}"
