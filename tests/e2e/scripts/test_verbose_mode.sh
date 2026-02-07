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

echo -e "${BLUE}Testing progress output...${NC}"

# Test 1: Step progress shows on stderr
echo -e "${YELLOW}  Test 1: Step progress on stderr${NC}"
STDERR_OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --dry-run 2>&1 >/dev/null)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze${NC}"
    exit 1
fi

# Check for step progress indicators on stderr
if ! echo "$STDERR_OUTPUT" | grep -q "Dry-run mode"; then
    echo -e "${RED}    ✗ No 'Dry-run mode' step in stderr${NC}"
    echo "stderr was: $STDERR_OUTPUT"
    exit 1
fi

if ! echo "$STDERR_OUTPUT" | grep -q "Found.*statement"; then
    echo -e "${RED}    ✗ No 'Found N statements' step in stderr${NC}"
    echo "stderr was: $STDERR_OUTPUT"
    exit 1
fi

if ! echo "$STDERR_OUTPUT" | grep -q "Analysis complete"; then
    echo -e "${RED}    ✗ No 'Analysis complete' step in stderr${NC}"
    echo "stderr was: $STDERR_OUTPUT"
    exit 1
fi

echo -e "${GREEN}    ✓ Step progress shows on stderr${NC}"

# Test 2: Stdout does not contain progress (only results)
echo -e "${YELLOW}  Test 2: Progress is separated from stdout${NC}"
STDOUT_OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --dry-run --format json 2>/dev/null)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze${NC}"
    exit 1
fi

# Stdout should be valid JSON (no progress mixed in)
if ! echo "$STDOUT_OUTPUT" | jq . > /dev/null 2>&1; then
    echo -e "${RED}    ✗ Stdout is not valid JSON (progress may be leaking to stdout)${NC}"
    exit 1
fi

if echo "$STDOUT_OUTPUT" | grep -q "Dry-run mode"; then
    echo -e "${RED}    ✗ Progress text leaked into stdout${NC}"
    exit 1
fi

echo -e "${GREEN}    ✓ Progress is on stderr only, stdout is clean${NC}"

# Test 3: Progress with multiple files
echo -e "${YELLOW}  Test 3: Progress with multiple files${NC}"

# Create temp directory with multiple migration files
TEMP_DIR=$(mktemp -d)
cp "$E2E_DIR/fixtures/postgres_test_migration.sql" "$TEMP_DIR/migration1.sql"
cp "$E2E_DIR/fixtures/postgres_test_migration.sql" "$TEMP_DIR/migration2.sql"
cp "$E2E_DIR/fixtures/postgres_test_migration.sql" "$TEMP_DIR/migration3.sql"

STDERR_OUTPUT=$("$TAPA_BIN" analyze "$TEMP_DIR" --dry-run 2>&1 >/dev/null)
STATUS=$?

# Cleanup
rm -rf "$TEMP_DIR"

if [ $STATUS -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze multiple files${NC}"
    exit 1
fi

# Should show statement count across all files
if ! echo "$STDERR_OUTPUT" | grep -q "Found.*statement"; then
    echo -e "${RED}    ✗ No statement count for multiple files${NC}"
    exit 1
fi

echo -e "${GREEN}    ✓ Progress works with multiple files${NC}"

# Test 4: Database connection step shows when connecting
echo -e "${YELLOW}  Test 4: Dry-run step indicator${NC}"
STDERR_OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --dry-run 2>&1 >/dev/null)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze${NC}"
    exit 1
fi

# Dry-run should show dry-run indicator
if ! echo "$STDERR_OUTPUT" | grep -q "Dry-run mode"; then
    echo -e "${RED}    ✗ No dry-run indicator${NC}"
    exit 1
fi

echo -e "${GREEN}    ✓ Dry-run step indicator present${NC}"

echo -e "${GREEN}Progress output E2E tests completed successfully!${NC}"
