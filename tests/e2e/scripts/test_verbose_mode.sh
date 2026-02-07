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

echo -e "${BLUE}Testing verbose mode...${NC}"

# Test 1: Verbose mode shows progress
echo -e "${YELLOW}  Test 1: Verbose mode shows progress indicators${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --dry-run --verbose 2>&1)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze with --verbose${NC}"
    exit 1
fi

# Check for progress indicators
if ! echo "$OUTPUT" | grep -q "Parsing files:"; then
    echo -e "${RED}    ✗ No 'Parsing files:' progress indicator${NC}"
    exit 1
fi

if ! echo "$OUTPUT" | grep -qE "\[[0-9]+/[0-9]+\]"; then
    echo -e "${RED}    ✗ No [X/Y] progress counter${NC}"
    exit 1
fi

echo -e "${GREEN}    ✓ Verbose mode shows progress indicators${NC}"

# Test 2: Non-verbose mode is silent
echo -e "${YELLOW}  Test 2: Non-verbose mode doesn't show progress${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --dry-run 2>&1)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze without --verbose${NC}"
    exit 1
fi

# Should NOT contain progress indicators in stderr
if echo "$OUTPUT" | grep -q "Parsing files:"; then
    echo -e "${RED}    ✗ Non-verbose mode showing progress (should be silent)${NC}"
    exit 1
fi

echo -e "${GREEN}    ✓ Non-verbose mode is silent during processing${NC}"

# Test 3: Verbose with multiple files
echo -e "${YELLOW}  Test 3: Verbose mode with multiple files${NC}"

# Create temp directory with multiple migration files
TEMP_DIR=$(mktemp -d)
cp "$E2E_DIR/fixtures/postgres_test_migration.sql" "$TEMP_DIR/migration1.sql"
cp "$E2E_DIR/fixtures/postgres_test_migration.sql" "$TEMP_DIR/migration2.sql"
cp "$E2E_DIR/fixtures/postgres_test_migration.sql" "$TEMP_DIR/migration3.sql"

OUTPUT=$("$TAPA_BIN" analyze "$TEMP_DIR" --dry-run --verbose 2>&1)
STATUS=$?

# Cleanup
rm -rf "$TEMP_DIR"

if [ $STATUS -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze multiple files${NC}"
    exit 1
fi

# Should show progress for multiple files
if ! echo "$OUTPUT" | grep -qE "\[[1-3]/3\]"; then
    echo -e "${RED}    ✗ No progress counter for multiple files${NC}"
    exit 1
fi

echo -e "${GREEN}    ✓ Verbose mode shows progress for multiple files${NC}"

# Test 4: Progress timing information
echo -e "${YELLOW}  Test 4: Progress shows elapsed time${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/postgres_test_migration.sql" --dry-run --verbose 2>&1)
if [ $? -ne 0 ]; then
    echo -e "${RED}    ✗ Failed to analyze${NC}"
    exit 1
fi

# Should show timing like "(0.5s)"
if ! echo "$OUTPUT" | grep -qE "\([0-9.]+s\)"; then
    echo -e "${RED}    ✗ No timing information in progress${NC}"
    exit 1
fi

echo -e "${GREEN}    ✓ Progress shows elapsed time${NC}"

echo -e "${GREEN}Verbose mode E2E tests completed successfully!${NC}"
