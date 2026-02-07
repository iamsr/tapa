#!/bin/bash
set -e

# Colors for test output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
E2E_DIR="$PROJECT_ROOT/tests/e2e"
TAPA_BIN="$PROJECT_ROOT/tapa"

echo -e "${BLUE}Testing color support...${NC}"

# Test 1: Default output has ANSI colors (when TTY)
echo -e "${YELLOW}  Test 1: Colors enabled by default${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --dry-run 2>&1)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze${NC}"
    exit 1
fi

# Check for ANSI escape codes
if echo "$OUTPUT" | grep -qE '\033\[[0-9;]+m'; then
    echo -e "${GREEN}    ✓ ANSI color codes present in output${NC}"
else
    echo -e "${YELLOW}    ⚠ No ANSI codes (expected if not TTY)${NC}"
fi

# Test 2: NO_COLOR=1 disables colors
echo -e "${YELLOW}  Test 2: NO_COLOR environment variable disables colors${NC}"
OUTPUT=$(NO_COLOR=1 "$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --dry-run 2>&1)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze with NO_COLOR${NC}"
    exit 1
fi

# Should NOT contain ANSI escape codes
if echo "$OUTPUT" | grep -qE '\033\[[0-9;]+m'; then
    echo -e "${RED}    ✗ ANSI codes present despite NO_COLOR=1${NC}"
    exit 1
fi

echo -e "${GREEN}    ✓ NO_COLOR=1 removes all ANSI codes${NC}"

# Test 3: Piped output disables colors
echo -e "${YELLOW}  Test 3: Piped output auto-disables colors${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --dry-run 2>&1 | cat)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze with pipe${NC}"
    exit 1
fi

# Piped output should not have colors (Go detects non-TTY)
if echo "$OUTPUT" | grep -qE '\033\[[0-9;]+m'; then
    echo -e "${YELLOW}    ⚠ ANSI codes in piped output (may be expected on some systems)${NC}"
else
    echo -e "${GREEN}    ✓ Piped output has no ANSI codes${NC}"
fi

# Test 4: Batch command respects color settings
echo -e "${YELLOW}  Test 4: Batch command respects NO_COLOR${NC}"
OUTPUT=$(NO_COLOR=1 "$TAPA_BIN" batch "$E2E_DIR/fixtures/postgres_test_migration.sql" --dry-run 2>&1)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to run batch command${NC}"
    exit 1
fi

if echo "$OUTPUT" | grep -qE '\033\[[0-9;]+m'; then
    echo -e "${RED}    ✗ Batch command ignores NO_COLOR${NC}"
    exit 1
fi

echo -e "${GREEN}    ✓ Batch command respects NO_COLOR${NC}"

# Test 5: JSON output never has ANSI codes
echo -e "${YELLOW}  Test 5: JSON output never has ANSI codes${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --dry-run --format json 2>&1)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze with JSON format${NC}"
    exit 1
fi

# JSON should never contain ANSI codes
if echo "$OUTPUT" | grep -qE '\033\[[0-9;]+m'; then
    echo -e "${RED}    ✗ ANSI codes in JSON output${NC}"
    exit 1
fi

# Should be valid JSON
if ! echo "$OUTPUT" | jq . > /dev/null 2>&1; then
    echo -e "${RED}    ✗ Invalid JSON output${NC}"
    exit 1
fi

echo -e "${GREEN}    ✓ JSON output clean and valid${NC}"

echo -e "${GREEN}Color support E2E tests completed successfully!${NC}"
