# CI/CD Integration Tests

Local test scripts for validating GitHub Action and GitLab CI integrations.

## Purpose

These scripts allow you to test the DMA CI/CD integrations locally before committing changes or deploying to production CI environments.

## Prerequisites

### For GitHub Action Testing
- [act](https://github.com/nektos/act) - Run GitHub Actions locally
  ```bash
  brew install act  # macOS
  ```

### For GitLab CI Testing
- Go 1.21+
- [jq](https://stedolan.github.io/jq/) - JSON processor
  ```bash
  brew install jq  # macOS
  ```

## Usage

### Test GitHub Action
```bash
./test-ci/test-github-action.sh
```

**What it validates:**
- GitHub Action workflow syntax
- DMA action execution with PostgreSQL
- DMA action execution with MySQL
- Action inputs and outputs

### Test GitLab CI Script
```bash
./test-ci/test-gitlab-ci.sh
```

**What it validates:**
- GitLab CI script execution
- Environment variable handling
- Report generation (JSON and Markdown)
- Output artifact creation

## What Each Script Does

### `test-github-action.sh`
1. Checks if `act` is installed
2. Creates test migration files
3. Creates temporary GitHub workflow
4. Runs workflow using `act`
5. Validates action execution
6. Cleans up test files

### `test-gitlab-ci.sh`
1. Checks prerequisites (Go, jq)
2. Creates test migration files
3. Sets DMA environment variables
4. Executes `.gitlab/dma-analyzer.sh`
5. Verifies JSON and Markdown reports
6. Displays report summary
7. Cleans up test files

## Notes

- Both scripts use temporary test files and clean up after execution
- Scripts exit on first error (`set -e`)
- Test migrations are simple ALTER TABLE statements for validation
- Reports are generated in the current directory during testing
