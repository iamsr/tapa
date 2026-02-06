# GitHub Action Usage Guide

## Overview

The DMA Analyzer GitHub Action automatically analyzes database migrations in pull requests, providing risk assessment and recommendations directly in PR comments.

## Setup

### 1. Add the workflow file

Create `.github/workflows/dma-pr-check.yml`:

```yaml
name: Migration Analysis

on:
  pull_request:
    paths:
      - 'db/migrations/**'
      - 'migrations/**'

jobs:
  analyze:
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Analyze migrations
        uses: ./.github/actions/tapa-analyzer
        with:
          migration-path: 'migrations/'
          db-type: 'postgresql'
          fail-on-risk: 'high'
          github-token: ${{ secrets.GITHUB_TOKEN }}
          comprehensive: 'true'
```

### 2. Ensure Go is available

The action requires Go to be installed in the workflow (as shown with `actions/setup-go@v4`).

### 3. Grant permissions

Add to your workflow file if needed:

```yaml
permissions:
  contents: read
  pull-requests: write
```

## Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `migration-path` | Yes | - | Path to migration files directory |
| `db-url` | No | - | Database connection string for live analysis |
| `db-type` | No | `postgresql` | Database type (`postgresql` or `mysql`) |
| `fail-on-risk` | No | - | Fail if risk exceeds level (`medium`, `high`, `critical`) |
| `github-token` | Yes | - | GitHub token for PR comments (`${{ secrets.GITHUB_TOKEN }}`) |
| `comprehensive` | No | `false` | Enable comprehensive analysis with statistics |

## Outputs

| Output | Description |
|--------|-------------|
| `risk-level` | Maximum risk level found (`low`, `medium`, `high`, `critical`) |
| `total-operations` | Total number of database operations analyzed |

## Examples

### Example 1: Block high-risk migrations

```yaml
- name: Analyze migrations
  uses: ./.github/actions/tapa-analyzer
  with:
    migration-path: 'db/migrations/'
    db-type: 'postgresql'
    fail-on-risk: 'high'
    github-token: ${{ secrets.GITHUB_TOKEN }}
```

This will fail the workflow if any migration contains operations with `high` or `critical` risk.

### Example 2: With database connection

```yaml
- name: Analyze migrations
  uses: ./.github/actions/tapa-analyzer
  with:
    migration-path: 'migrations/'
    db-url: ${{ secrets.DATABASE_URL }}
    db-type: 'postgresql'
    comprehensive: 'true'
    github-token: ${{ secrets.GITHUB_TOKEN }}
```

Connect to a real database for accurate table statistics and lock analysis.

### Example 3: MySQL migrations

```yaml
- name: Analyze migrations
  uses: ./.github/actions/tapa-analyzer
  with:
    migration-path: 'db/migrations/'
    db-type: 'mysql'
    fail-on-risk: 'critical'
    github-token: ${{ secrets.GITHUB_TOKEN }}
    comprehensive: 'true'
```

Analyze MySQL migrations with algorithm detection and `pt-online-schema-change` recommendations.

## What the Action Does

1. **Installs DMA** - Downloads and installs the latest version of DMA
2. **Analyzes migrations** - Runs DMA against your migration files
3. **Posts PR comment** - Creates or updates a comment with:
   - Summary of operations and risk levels
   - Table of all operations with risk indicators
   - Recommendations for high-risk operations
   - Safer alternatives when available
4. **Fails workflow** - If `fail-on-risk` is set and exceeded

## PR Comment Format

The action posts a formatted comment on your PR:

```
## 🔍 Migration Analysis Results

**Total Operations:** 5
**High Risk Operations:** 1

### Operations

| Operation | Table | Risk | Lock Type | Estimated Time |
|-----------|-------|------|-----------|----------------|
| 🟢 CREATE INDEX | users | 45 | SHARE | 12s |
| 🔴 ADD COLUMN | orders | 78 | ACCESS EXCLUSIVE | 45s |

### ⚠️ High Risk Operations

**ADD COLUMN on `orders`** (Risk: 78)
Recommendations:
- Add column with DEFAULT requires table rewrite
- Consider adding column without DEFAULT, then backfill

### 💡 Safer Alternatives Available

**ADD COLUMN on `orders`**
- Two-step approach: Add column without DEFAULT, then SET DEFAULT separately
```

## Troubleshooting

**Action fails with "DMA not found"**: Ensure Go is installed with `actions/setup-go@v4`

**No PR comment posted**: Check that `github-token` is provided and workflow has `pull-requests: write` permission

**"Cannot find module" errors**: The action uses bundled dependencies in `dist/index.js` - no need to run `npm install`
