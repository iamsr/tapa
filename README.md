# Table Alteration Planning Assistant (TAPA)

[![codecov](https://codecov.io/gh/iamsr/tapa/branch/main/graph/badge.svg)](https://codecov.io/gh/iamsr/tapa)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**[https://tapa.com](https://tapa-rho.vercel.app/)**

A static analysis tool that predicts the production impact of database migrations before execution. TAPA analyzes migration files and provides risk assessments, lock predictions, time estimates, and safer alternatives.

**Supported Databases:** PostgreSQL 9.6+, MySQL 5.7+

## Features

### Migration Analysis

- **Lock Detection**: Predicts lock types (ACCESS EXCLUSIVE, SHARE, etc.) and durations
- **Risk Scoring**: Calculates risk scores (0-100) with categorization (LOW, MEDIUM, HIGH, CRITICAL)
- **Time Estimation**: Estimates execution time including table rewrites and index builds
- **Dependency Analysis**: Identifies breaking changes (indexes, views, foreign keys)
- **Safe Alternatives**: Generates multi-step alternatives for high-risk operations
- **Migration Batching**: Groups operations by risk for safer deployment

### Database Support

- **PostgreSQL**: Full DDL parsing with pg_query, supports CONCURRENTLY operations
- **MySQL**: Native vitess parser with ALGORITHM/LOCK clause detection
- **pt-online-schema-change**: Automatic command generation for high-risk MySQL operations
- **Schema Introspection**: Queries live database metadata for accurate analysis

### CI/CD Integration

- **GitHub Actions**: Automatic PR analysis with risk-based blocking and comments
- **GitLab CI**: Pipeline integration with JSON and Markdown reports
- **JSON Output**: Machine-readable format for custom automation workflows

### Advanced Features (Comprehensive Mode)

Enable comprehensive analysis with the `--comprehensive` flag for production-ready insights:

```bash
tapa analyze migrations/ --db $DATABASE_URL --comprehensive
```

Features include:

- **Disk Space Requirements**: Calculate space needed before, during, and after migration
- **Rollback Analysis**: Determine reversibility and get auto-generated rollback scripts
- **Data Migration Detection**: Find hidden UPDATE/INSERT/DELETE operations with time estimates

See [Advanced Features Guide](docs/advanced-features.md) for details.

## Installation

```bash
go install github.com/iamsr/tapa/cmd/tapa@latest
```

## Quick Start

### Analyze Migrations

**PostgreSQL:**

```bash
# Single file
tapa analyze migrations/001_add_column.sql --db postgres://localhost/mydb

# Directory
tapa analyze migrations/ --db postgres://user:pass@host/dbname

# Comprehensive analysis with all features
tapa analyze migrations/ --db $DATABASE_URL --comprehensive
```

**MySQL:**

```bash
# Basic analysis
tapa analyze migrations/001_add_index.sql --db-type mysql --db mysql://root@localhost/mydb

# With pt-osc recommendations
tapa analyze migrations/ --db-type mysql --db mysql://localhost/mydb

# Dry run (no database connection)
tapa analyze migrations/ --db-type mysql --dry-run
```

### Output Formats

**Human-readable (default):**

TAPA provides beautiful, color-coded output with visual summary cards:

```bash
tapa analyze migrations/001_migration.sql --db $DATABASE_URL
```

Example output:

```
  ✓ Dry-run mode (no database connection)
  Parsing migration file(s)...
  ✓ Found 6 statements
  Analyzing operations...
  ✓ Analysis complete

╭─ ANALYSIS RESULTS ────────────────────────────────────────────────────────╮
│ Risk Score                                                               │
│ ███████████████████░░░░░░░░░  70/100                                     │
│ Status: HIGH RISK 🟠                                                     │
│                                                                          │
│ Est. Time: 4.3m                                                          │
│                                                                          │
│ Risk Breakdown:                                                          │
│     ├── Low      ▪ 1                                                     │
│     ├── Medium   ▪▪ 2                                                    │
│     ├── High     ▪▪▪ 3                                                   │
│     └── Critical  0                                                      │
│                                                                          │
│ Compatibility:                                                           │
│   ✓ All operations backward compatible (6/6)                             │
│   ✓ No breaking changes                                                  │
│   ⚠ Requires maintenance window                                         │
╰──────────────────────────────────────────────────────────────────────────╯
```

Features:

- Visual progress bars with risk-based coloring
- Tree-style risk breakdown with operation counts
- Emoji status indicators for quick assessment
- Compatibility checks at a glance

**JSON (for CI/CD):**

```bash
tapa analyze migrations/ --db $DATABASE_URL --format json > report.json
```

### Migration Batching

Generate safer deployment strategies by grouping operations by risk level:

```bash
# Analyze and generate batching strategy
tapa batch migrations/ --db-type postgresql

# Output in JSON format
tapa batch migrations/ --format json > batches.json
```

Features:

- Risk-based operation grouping
- Automatic prerequisite detection
- Parallel execution recommendations
- Per-batch time estimates

See [Batching Guide](docs/batching-guide.md) for details.

### Progress Output

TAPA displays step-by-step progress on stderr during analysis:

```bash
tapa analyze migrations/ --db $DATABASE_URL
```

Progress output (on stderr):

```
  Connecting to database...
  ✓ Connected to Postgresql
  Parsing migration file(s)...
  ✓ Found 6 statements
  Analyzing operations...
  ✓ Analysis complete
```

This keeps stdout clean for JSON output while showing progress:

```bash
tapa analyze migrations/ --format json > report.json  # Progress on stderr, JSON on stdout
```

Disable emojis in progress output:

```bash
TAPA_NO_EMOJI=1 tapa analyze migrations/
```

### Color-Coded Output

TAPA automatically displays risk levels and lock types in color for better visibility:

- **Risk Levels:** Green (LOW), Yellow (MEDIUM), Orange (HIGH), Red (CRITICAL)
- **Lock Types:** Color-coded based on severity

Disable colors or emojis if needed:

```bash
NO_COLOR=1 tapa analyze migrations/       # Disable colors
TAPA_NO_EMOJI=1 tapa analyze migrations/  # Replace emojis with text
```

### CI/CD Integration

**GitHub Actions:**

```yaml
- uses: ./.github/actions/tapa-analyzer
  with:
    migration-path: "migrations/"
    db-type: "postgresql"
    fail-on-risk: "high"
    github-token: ${{ secrets.GITHUB_TOKEN }}
```

**GitLab CI:**

```yaml
migration-analysis:
  script:
    - ./.gitlab/tapa-analyzer.sh migrations/
  variables:
    DMA_DB_TYPE: postgresql
    DMA_FAIL_ON_RISK: high
```

## Documentation

- **[Architecture Guide](docs/architecture.md)** - Comprehensive system design, algorithms, and extension points
- [Migration Batching Guide](docs/batching-guide.md) - Safer incremental deployment strategies
- [MySQL Support Guide](docs/mysql-support.md) - MySQL-specific features and pt-osc integration
- [GitHub Actions Setup](docs/github-action-usage.md) - Automated PR analysis
- [GitLab CI Setup](docs/gitlab-ci-usage.md) - Pipeline integration

Full documentation available in [docs/](docs/).

## Architecture

TAPA uses a multi-stage pipeline architecture for comprehensive migration analysis:

```
cmd/tapa/              # CLI entry point
internal/
  parser/             # SQL parsing (PostgreSQL pg_query, MySQL Vitess)
  analyzer/           # Lock detection, risk scoring, time estimation
  introspector/       # Live database metadata queries
  db/                 # Database connection management
  batcher/            # Risk-based operation grouping
  ui/                 # Progress bars, summary cards, visual output
  output/             # Multi-format output (table, JSON, YAML)
pkg/models/           # Core data structures
```

For detailed architecture documentation including data flows, algorithms, and extension points, see the **[Architecture Guide](docs/architecture.md)**.

## Development

**Requirements:**

- Go 1.21+
- PostgreSQL 9.6+ or MySQL 5.7+ (for integration tests)

**Build:**

```bash
go build ./cmd/tapa
```

**Test:**

TAPA has comprehensive test coverage across multiple levels:

### Unit Tests

Located in `tests/unit/` and alongside code in internal packages:

```bash
# Run all unit tests
go test ./... -short

# Run specific package
go test ./internal/analyzer -v
```

### Integration Tests

Integration tests are co-located with code (see `tests/integration/README.md`):

```bash
# Requires Docker (PostgreSQL + MySQL)
cd tests/e2e && docker-compose up -d
go test ./internal/analyzer/postgres -v -run Integration
go test ./internal/analyzer/mysql -v -run Integration
cd tests/e2e && docker-compose down -v
```

### End-to-End Tests

Located in `tests/e2e/`:

```bash
cd tests/e2e
./run_tests.sh
```

This runs the full E2E suite including:

- PostgreSQL integration tests
- MySQL integration tests
- Batch command tests

### CI/CD Tests

CI integration test scripts are in `tests/ci/`:

```bash
# GitHub Actions
bash tests/ci/test-github-action.sh

# GitLab CI
bash tests/ci/test-gitlab-ci.sh
```

**Test with coverage:**

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Apache License 2.0 - see [LICENSE](LICENSE)
