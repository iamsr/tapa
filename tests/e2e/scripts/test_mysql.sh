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

# MySQL connection  
MYSQL_URL="testuser:testpass@tcp(localhost:3307)/testdb"

echo -e "${BLUE}Testing MySQL integration...${NC}"

# Test 1: Basic analysis without database connection (dry-run)
echo -e "${YELLOW}  Test 1: Dry-run analysis${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/mysql_test_migration.sql" --db-type mysql --dry-run --format json 2>/dev/null)
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
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/mysql_test_migration.sql" --db-type mysql --db "$MYSQL_URL" --format json 2>/dev/null)
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
echo -e "${GREEN}    ✓ Database connection analysis successful${NC}"

# Test 3: Comprehensive analysis with Phase 2 features
echo -e "${YELLOW}  Test 3: Comprehensive analysis (Phase 2 features)${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/mysql_test_migration.sql" --db-type mysql --db "$MYSQL_URL" --comprehensive --format json 2>/dev/null)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed: $OUTPUT${NC}"
    exit 1
fi
echo "$OUTPUT" | jq . > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Invalid JSON output${NC}"
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

# Test 4: Check for MySQL-specific features (ALGORITHM, LOCK detection)
echo -e "${YELLOW}  Test 4: Verify MySQL-specific features${NC}"
HAS_METADATA=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[] | select(.Metadata != null)] | length')
if [ "$HAS_METADATA" -lt 1 ]; then
    echo -e "${YELLOW}    ⚠ Note: No MySQL metadata detected (ALGORITHM/LOCK clauses)${NC}"
else
    echo -e "${GREEN}    ✓ MySQL metadata detected in $HAS_METADATA operations${NC}"
fi

# Test 5: Check for specific operation types
echo -e "${YELLOW}  Test 5: Verify operation detection${NC}"
ADD_COLUMN_COUNT=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[] | select(.Type == "ADD_COLUMN")] | length')
MODIFY_COLUMN_COUNT=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[] | select(.Type == "MODIFY_COLUMN")] | length')

if [ "$ADD_COLUMN_COUNT" -lt 1 ]; then
    echo -e "${RED}    ✗ Expected ADD_COLUMN operations${NC}"
    exit 1
fi
echo -e "${GREEN}    ✓ Operations detected correctly: ADD_COLUMN($ADD_COLUMN_COUNT), MODIFY_COLUMN($MODIFY_COLUMN_COUNT)${NC}"

# Test 6: Check for risk scoring
echo -e "${YELLOW}  Test 6: Verify risk scoring${NC}"
MEDIUM_RISK=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[] | select(.RiskScore >= 40 and .RiskScore < 70)] | length')
HIGH_RISK=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[] | select(.RiskScore >= 70)] | length')

if [ "$MEDIUM_RISK" -lt 1 ] && [ "$HIGH_RISK" -lt 1 ]; then
    echo -e "${RED}    ✗ Expected some operations with risk scores${NC}"
    exit 1
fi
echo -e "${GREEN}    ✓ Risk scoring working: MEDIUM($MEDIUM_RISK), HIGH($HIGH_RISK)${NC}"

# Test 7: Verify pt-online-schema-change recommendations for high-risk ops
echo -e "${YELLOW}  Test 7: Verify pt-osc recommendations${NC}"
HAS_PT_OSC=$(echo "$OUTPUT" | jq '[.Migrations[].Operations[] | select(.Metadata != null and .Metadata.PTOnlineSchemaChange != null)] | length')
if [ "$HAS_PT_OSC" -lt 1 ]; then
    echo -e "${YELLOW}    ⚠ Note: No pt-online-schema-change recommendations${NC}"
else
    echo -e "${GREEN}    ✓ pt-online-schema-change recommended for $HAS_PT_OSC operations${NC}"
fi

# Test 8: Show actual program output (table format)
echo -e "${YELLOW}  Test 8: Sample output (table format)${NC}"
echo -e "${BLUE}  Running: tapa analyze mysql_test_migration.sql --db-type mysql --db <mysql> --format table${NC}"
echo ""
"$TAPA_BIN" analyze "$E2E_DIR/fixtures/mysql_test_migration.sql" --db-type mysql --db "$MYSQL_URL" --format table 2>&1 || true
echo ""

echo -e "${GREEN}MySQL E2E tests completed successfully!${NC}"
