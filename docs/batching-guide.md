# Migration Batching Guide

## Overview

TAPA's batching feature helps you split complex migrations into safer, smaller batches for incremental deployment. Operations are grouped by risk level to minimize production impact.

## Usage

### Generate Batching Strategy

```bash
tapa batch migrations/ --db-type postgresql
```

### With Database Connection

```bash
tapa batch migrations/ --db "postgresql://user:pass@localhost:5432/db"
```

### Output Formats

**Table Format (default):**
```bash
tapa batch migrations/ --format table
```

**JSON Format:**
```bash
tapa batch migrations/ --format json > batches.json
```

**YAML Format:**
```bash
tapa batch migrations/ --format yaml
```

## Batching Strategy

### Risk-Based Grouping

1. **Batch 1: Low-Risk Operations**
   - Risk Score: 0-25
   - Can run in parallel
   - Safe for immediate deployment
   - Examples: Adding nullable columns, creating indexes with CONCURRENTLY

2. **Batch 2: Medium-Risk Operations**
   - Risk Score: 26-50
   - Execute sequentially
   - Deploy during low-traffic periods
   - Examples: Adding columns with defaults, modifying column types

3. **Batch 3+: High/Critical-Risk Operations**
   - Risk Score: 51-100
   - One operation per batch
   - Requires maintenance window
   - Examples: Adding NOT NULL constraints, dropping columns

### Prerequisites

Each batch declares prerequisites (previous batches that must complete first). This ensures safe sequential execution.

## Example Output

```
Migration Batching Strategy
================================================================================

Summary:
  Total Operations: 5
  Total Batches: 3
  Estimated Total Time: 2.5s
  Max Risk Level: CRITICAL

Batch #1 (LOW):
  Operations: 2
  Risk Score: 10/100
  Estimated Time: 0.1s
  Parallel Execution: true
  Rationale: Low-risk operations can be deployed immediately

  Operations:
    1. ADD_COLUMN on users (Risk: 10)
    2. CREATE_INDEX on orders (Risk: 10)

Batch #2 (MEDIUM):
  Operations: 2
  Risk Score: 40/100
  Estimated Time: 1.2s
  Parallel Execution: false
  Prerequisites: Batches [1]
  Rationale: Medium-risk operations should be deployed during low-traffic periods

  Operations:
    1. ADD_COLUMN on products (Risk: 40)
    2. ALTER_COLUMN on users (Risk: 40)

Batch #3 (CRITICAL):
  Operations: 1
  Risk Score: 80/100
  Estimated Time: 1.2s
  Parallel Execution: false
  Prerequisites: Batches [1, 2]
  Rationale: High-risk operation requires maintenance window

  Operations:
    1. ADD_COLUMN on users (Risk: 80)

Recommendations:
  💡 Split into 3 batches for safer deployment
  ⚠️  Batch 3 (CRITICAL): Deploy during maintenance window
```

## Integration with CI/CD

### GitHub Actions

```yaml
- name: Generate Batching Strategy
  run: tapa batch migrations/ --format json > batches.json

- name: Upload Batches
  uses: actions/upload-artifact@v3
  with:
    name: migration-batches
    path: batches.json
```

### Deployment Pipeline

```bash
# Deploy batch by batch
for batch_num in $(jq '.Strategy.TotalBatches' batches.json); do
  echo "Deploying batch $batch_num..."
  # Extract and execute operations for this batch
  # Add monitoring and rollback logic
done
```

## Best Practices

1. **Review Batches**: Always review the generated strategy before deployment
2. **Test Each Batch**: Test batches individually in staging environment
3. **Monitor Between Batches**: Check metrics after each batch completes
4. **Keep Rollback Plans**: Prepare rollback scripts for each batch
5. **Use Maintenance Windows**: Schedule high-risk batches during low traffic

## See Also

- [Comprehensive Analysis Guide](comprehensive-analysis.md)
- [Risk Scoring](risk-scoring.md)
- [Alternative Strategies](alternatives.md)
