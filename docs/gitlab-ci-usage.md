# GitLab CI Integration

## Overview

Integrate DMA into your GitLab CI pipeline to automatically analyze migrations on merge requests.

## Setup

1. **Add the script to your repository:**

Copy `.gitlab/tapa-analyzer.sh` to your repository.

2. **Update `.gitlab-ci.yml`:**

```yaml
stages:
  - test

migration-analysis:
  stage: test
  image: golang:1.21
  before_script:
    - apt-get update && apt-get install -y jq
  script:
    - ./.gitlab/tapa-analyzer.sh migrations/
  variables:
    DMA_DB_TYPE: postgresql
    DMA_COMPREHENSIVE: "true"
    DMA_FAIL_ON_RISK: high
  artifacts:
    reports:
      junit: dma-report.json
    paths:
      - dma-report.json
      - dma-report.md
    expire_in: 1 week
  only:
    - merge_requests
    - main
```

3. **Commit and push**

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DMA_DB_TYPE` | No | `postgresql` | Database type |
| `DMA_DB_URL` | No | - | Database connection (uses dry-run if omitted) |
| `DMA_FAIL_ON_RISK` | No | - | Fail if risk exceeds threshold |
| `DMA_COMPREHENSIVE` | No | `true` | Enable all features |

## Examples

### With database connection

```yaml
migration-analysis:
  script:
    - ./.gitlab/tapa-analyzer.sh migrations/
  variables:
    DMA_DB_URL: $DATABASE_URL
    DMA_DB_TYPE: mysql
```

### Fail on medium risk

```yaml
migration-analysis:
  script:
    - ./.gitlab/tapa-analyzer.sh migrations/
  variables:
    DMA_FAIL_ON_RISK: medium
```

### Custom migration path

```yaml
migration-analysis:
  script:
    - ./.gitlab/tapa-analyzer.sh db/schema/migrations/
```

## Viewing Reports

Reports are saved as artifacts:
- `dma-report.json` - Full JSON output
- `dma-report.md` - Markdown summary

Access via GitLab UI: Pipeline → Jobs → Browse → dma-report.md
