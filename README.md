# Database Migration Analyzer (DMA)

[![CI](https://github.com/yourusername/dma/workflows/CI/badge.svg)](https://github.com/yourusername/dma/actions)
[![codecov](https://codecov.io/gh/yourusername/dma/branch/main/graph/badge.svg)](https://codecov.io/gh/yourusername/dma)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Static analysis tool that predicts production impact of database migrations before execution.

**Supports: PostgreSQL 9.6+, MySQL 5.7+**

## Features

### Analysis (Phase 1 & 2)
- **Lock Analysis**: Predicts lock types and durations
- **Table Rewrite Detection**: Estimates time for full table rewrites
- **Index Build Estimation**: Calculates index creation impact
- **Backward Compatibility**: Detects breaking schema changes
- **Risk Scoring**: Actionable recommendations with risk levels

### Database Support (Phase 3)
- **PostgreSQL 9.6+**: Full support for all migration operations
- **MySQL 5.7+**: Native parser with ALGORITHM/LOCK detection
- **pt-online-schema-change**: Automatic integration for safe MySQL migrations
- **Schema Introspection**: Live database metadata queries

### CI/CD Integration (Phase 3)
- **GitHub Actions**: Automated PR analysis with blocking and comments
- **GitLab CI**: Shell script integration with artifacts and reports
- **JSON Output**: Machine-readable results for custom workflows

## Installation

```bash
go install github.com/yourusername/dma/cmd/dma@latest
```

## Quick Start

### PostgreSQL
```bash
# Analyze single migration
dma analyze migrations/001_add_column.sql --db postgres://localhost/mydb

# Analyze directory
dma analyze migrations/ --db postgres://user:pass@host/dbname

# Output JSON for CI/CD
dma analyze migrations/ --db $DATABASE_URL --format json
```

### MySQL
```bash
# Analyze MySQL migration
dma analyze migrations/001_add_index.sql --db mysql://root@localhost/mydb

# With pt-online-schema-change
dma analyze migrations/ --db mysql://localhost/mydb --use-pt-osc

# Dry run mode
dma analyze migrations/ --db mysql://localhost/mydb --dry-run
```

### CI/CD Usage

#### GitHub Actions
```yaml
- uses: yourusername/dma-action@v1
  with:
    database_url: ${{ secrets.DATABASE_URL }}
    migrations_path: ./migrations
    fail_on_high_risk: true
```

#### GitLab CI
```yaml
dma-analysis:
  script:
    - dma analyze migrations/ --db $DATABASE_URL --format json
  artifacts:
    reports:
      dma: dma-report.json
```

## Documentation

- [MySQL Support](docs/mysql-support.md)
- [GitHub Action Usage](docs/github-action-usage.md)
- [GitLab CI Usage](docs/gitlab-ci-usage.md)
- [Phase 3 Changelog](docs/phase3-changelog.md)

See [docs/](docs/) for comprehensive documentation.

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache License 2.0 - see [LICENSE](LICENSE)
