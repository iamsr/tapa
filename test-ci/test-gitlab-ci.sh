#!/bin/bash
set -e

echo "🧪 Testing GitLab CI script for DMA locally..."

# Check prerequisites
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go first."
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo "❌ jq is not installed. Please install it first:"
    echo "   brew install jq  # macOS"
    exit 1
fi

# Create test directory
TEST_DIR="test-migrations"
mkdir -p "$TEST_DIR"

echo "📝 Creating test migration..."
cat > "$TEST_DIR/001_test_migration.sql" <<'EOF'
-- Test migration for GitLab CI
ALTER TABLE users ADD COLUMN email VARCHAR(255) DEFAULT 'unknown';
CREATE INDEX idx_email ON users(email);
EOF

echo "⚙️  Setting up environment variables..."
export DMA_MIGRATION_PATH="$TEST_DIR"
export DMA_DATABASE_TYPE="postgresql"
export DMA_OUTPUT_FORMAT="json"
export DMA_REPORT_FILE="dma-report.json"
export DMA_MARKDOWN_FILE="dma-report.md"

echo "🚀 Running GitLab CI script..."
bash .gitlab/dma-analyzer.sh

echo "✅ GitLab CI script completed!"
echo ""

echo "🔍 Verifying outputs..."
if [ -f "dma-report.json" ]; then
    echo "✅ JSON report generated"
    echo "   Summary: $(jq -r '.summary' dma-report.json 2>/dev/null || echo 'Could not parse summary')"
else
    echo "❌ JSON report not found"
fi

if [ -f "dma-report.md" ]; then
    echo "✅ Markdown report generated"
    echo "   First few lines:"
    head -n 5 dma-report.md | sed 's/^/   /'
else
    echo "❌ Markdown report not found"
fi

echo ""
echo "🧹 Cleaning up..."
rm -rf "$TEST_DIR"
rm -f dma-report.json dma-report.md

echo "✨ Done!"
