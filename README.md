# Database Migration Analyzer (DMA)

[![CI](https://github.com/yourusername/dma/workflows/CI/badge.svg)](https://github.com/yourusername/dma/actions)
[![codecov](https://codecov.io/gh/yourusername/dma/branch/main/graph/badge.svg)](https://codecov.io/gh/yourusername/dma)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Static analysis tool that predicts production impact of database migrations before execution.

## Features

- **Lock Analysis**: Predicts lock types and durations
- **Table Rewrite Detection**: Estimates time for full table rewrites
- **Index Build Estimation**: Calculates index creation impact
- **Backward Compatibility**: Detects breaking schema changes
- **Risk Scoring**: Actionable recommendations with risk levels
- **CI/CD Integration**: GitHub Actions, GitLab CI support

## Installation

```bash
go install github.com/yourusername/dma/cmd/dma@latest
```

## Quick Start

```bash
# Analyze single migration
dma analyze migrations/001_add_column.sql --db postgres://localhost/mydb

# Analyze directory
dma analyze migrations/ --db $DATABASE_URL

# Output JSON for CI/CD
dma analyze migrations/ --db $DB_URL --format json
```

## Documentation

See [docs/](docs/) for comprehensive documentation.

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache License 2.0 - see [LICENSE](LICENSE)
