# Database Migration Analyzer (DMA)

[![CI](https://github.com/iamsr/dma/workflows/CI/badge.svg)](https://github.com/iamsr/dma/actions)
[![codecov](https://codecov.io/gh/iamsr/dma/branch/main/graph/badge.svg)](https://codecov.io/gh/iamsr/dma)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

A static analysis tool that predicts the production impact of database migrations before execution. DMA analyzes migration files and provides risk assessments, lock predictions, time estimates, and safer alternatives.

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

## Installation

```bash
go install github.com/iamsr/dma/cmd/dma@latest
```

## Quick Start

### Analyze Migrations

**PostgreSQL:**
```bash
# Single file
dma analyze migrations/001_add_column.sql --db postgres://localhost/mydb

# Directory
dma analyze migrations/ --db postgres://user:pass@host/dbname

# Comprehensive analysis with all features
dma analyze migrations/ --db $DATABASE_URL --comprehensive
```

**MySQL:**
```bash
# Basic analysis
dma analyze migrations/001_add_index.sql --db-type mysql --db mysql://root@localhost/mydb

# With pt-osc recommendations
dma analyze migrations/ --db-type mysql --db mysql://localhost/mydb

# Dry run (no database connection)
dma analyze migrations/ --db-type mysql --dry-run
```

### Output Formats

**Human-readable (default):**
```bash
dma analyze migrations/001_migration.sql --db $DATABASE_URL
```

**JSON (for CI/CD):**
```bash
dma analyze migrations/ --db $DATABASE_URL --format json > report.json
```

### CI/CD Integration

**GitHub Actions:**
```yaml
- uses: ./.github/actions/dma-analyzer
  with:
    migration-path: 'migrations/'
    db-type: 'postgresql'
    fail-on-risk: 'high'
    github-token: ${{ secrets.GITHUB_TOKEN }}
```

**GitLab CI:**
```yaml
migration-analysis:
  script:
    - ./.gitlab/dma-analyzer.sh migrations/
  variables:
    DMA_DB_TYPE: postgresql
    DMA_FAIL_ON_RISK: high
```

## Documentation

- [MySQL Support Guide](docs/mysql-support.md) - MySQL-specific features and pt-osc integration
- [GitHub Actions Setup](docs/github-action-usage.md) - Automated PR analysis
- [GitLab CI Setup](docs/gitlab-ci-usage.md) - Pipeline integration

Full documentation available in [docs/](docs/).

## Architecture

```
cmd/dma/              # CLI entry point
internal/
  parser/             # SQL parsing (PostgreSQL, MySQL)
  analyzer/           # Lock detection, risk scoring
  introspector/       # Live database queries
  db/                 # Database connections
pkg/models/           # Core data structures
```

## Development

**Requirements:**
- Go 1.21+
- PostgreSQL 9.6+ or MySQL 5.7+ (for integration tests)

**Build:**
```bash
go build ./cmd/dma
```

**Test:**
```bash
go test ./...
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
