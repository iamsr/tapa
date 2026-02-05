#!/bin/bash
# Workaround for pg_query_go macOS compilation issue
# This script fixes the strchrnul redefinition error on macOS
#
# Issue: macOS provides strchrnul in its standard library, but pg_query_go
# also defines a static version, causing a compilation conflict.
#
# Solution: Change the condition to exclude macOS from the inline definition

set -e

MODCACHE=$(go env GOMODCACHE)
FILE="$MODCACHE/github.com/pganalyze/pg_query_go/v5@v5.1.0/parser/src_port_snprintf.c"

echo "Fixing pg_query_go for macOS..."
echo "Target file: $FILE"

if [ ! -f "$FILE" ]; then
    echo "Error: File not found. Have you run 'go mod download' first?"
    exit 1
fi

# Make the file writable if needed
chmod +w "$FILE" 2>/dev/null || true

# Create patched version
# Change line 372 from "#ifndef HAVE_STRCHRNUL" to "#if !defined(HAVE_STRCHRNUL) && !defined(__APPLE__)"
sed -i.bak 's|^#ifndef HAVE_STRCHRNUL$|#if !defined(HAVE_STRCHRNUL) \&\& !defined(__APPLE__)|' "$FILE"

# Verify the patch was applied
if grep -q "#if !defined(HAVE_STRCHRNUL) && !defined(__APPLE__)" "$FILE"; then
    echo "✓ Successfully patched pg_query_go"
else
    echo "✗ Patch failed - restoring backup"
    mv "$FILE.bak" "$FILE"
    exit 1
fi

# Clean Go cache to force rebuild
go clean -cache

echo "✓ Done! You can now run 'go test' or 'go build'"
