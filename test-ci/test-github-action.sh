#!/bin/bash
set -e

echo "🧪 Testing GitHub Action for DMA locally..."

# Check if act is installed
if ! command -v act &> /dev/null; then
    echo "❌ 'act' is not installed. Please install it first:"
    echo "   brew install act  # macOS"
    echo "   or visit: https://github.com/nektos/act"
    exit 1
fi

# Create test directory
TEST_DIR="test-migrations"
mkdir -p "$TEST_DIR"

echo "📝 Creating test migration..."
cat > "$TEST_DIR/001_test_migration.sql" <<'EOF'
-- Test migration for GitHub Action
ALTER TABLE users ADD COLUMN email VARCHAR(255) DEFAULT 'unknown';
CREATE INDEX idx_email ON users(email);
EOF

echo "⚙️  Creating test workflow..."
mkdir -p .github/workflows
cat > .github/workflows/test-dma.yml <<'EOF'
name: Test DMA
on: [push]

jobs:
  test-postgresql:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run DMA
        uses: ./.github/actions/dma-action
        with:
          migration-path: 'test-migrations'
          database-type: 'postgresql'

  test-mysql:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run DMA
        uses: ./.github/actions/dma-action
        with:
          migration-path: 'test-migrations'
          database-type: 'mysql'
EOF

echo "🚀 Running GitHub Action with act..."
act -j test-postgresql -j test-mysql

echo "✅ GitHub Action test completed!"
echo ""
echo "🧹 Cleaning up..."
rm -rf "$TEST_DIR"
rm .github/workflows/test-dma.yml

echo "✨ Done!"
