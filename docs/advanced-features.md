# Advanced Features Guide

TAPA includes five advanced features for comprehensive migration analysis. These features are enabled with the `--comprehensive` flag.

## Overview

| Feature | Description | Output |
|---------|-------------|--------|
| **Disk Space Requirements** | Calculates disk space needed before, during, and after migration | Space analysis with warnings |
| **Rollback Analysis** | Determines reversibility and generates rollback strategies | Rollback category and recovery steps |
| **Data Migration Detection** | Detects UPDATE/INSERT/DELETE operations in migrations | Performance estimates and batching advice |
| **Dry-Run Simulation** | (Coming soon) Executes migrations in temporary database | Runtime error detection |
| **Concurrency Impact** | (Coming soon) Predicts impact on concurrent queries | Lock analysis and optimal execution window |

## Usage

Enable advanced features with the `--comprehensive` flag:

```bash
tapa analyze migrations/ --db $DATABASE_URL --comprehensive
```

### Disk Space Requirements

Analyzes disk space usage throughout the migration lifecycle.

**Example Output:**

```
Disk Space Analysis:
─────────────────────────────────────
Current State:
  Table:   80.00 GB
  Indexes: 15.00 GB
  Total:   95.00 GB

Migration Requirements:
  Requires table rewrite: Yes
  Temporary table: 85.00 GB
  Peak disk usage: 196.00 GB

System Check:
  Available: 150.00 GB
  Required:  196.00 GB
  Status:    INSUFFICIENT SPACE
  Shortfall: 46.00 GB
```

**What it detects:**
- Table rewrite space requirements
- Index creation overhead
- Peak disk usage during migration
- Safety buffer (20%)
- Final state after migration

**Database-specific behavior:**
- PostgreSQL: 2x table size for rewrites (MVCC)
- MySQL: Depends on ALGORITHM (COPY requires 2x, INPLACE minimal)

### Rollback Analysis

Categorizes operations by reversibility and provides recovery strategies.

**Reversibility Categories:**

| Category | Score | Description | Example |
|----------|-------|-------------|---------|
| **SAFE** | 100 | Fully reversible | CREATE INDEX |
| **CONDITIONAL** | 50-75 | Reversible with conditions | DROP INDEX (need definition) |
| **DATA LOSS** | 25-50 | Causes data loss | Type conversion (NUMERIC→INT) |
| **IRREVERSIBLE** | 0 | Cannot be rolled back | DROP COLUMN, DROP TABLE |

**Example Output:**

```
Rollback Analysis:
─────────────────────────────────────
Category: DATA LOSS
Reversibility Score: 25/100
Reversible: No
Reason: Type conversion may cause precision loss

Recovery Strategy:
  Method: backup_restore
  Estimated RTO: Depends on database size
  Steps:
    1. Restore database from backup taken before migration
    2. Verify data integrity
    3. Consider alternative: use multi-column approach
```

**Auto-generated rollback scripts:**

For SAFE operations, TAPA generates rollback SQL:

```sql
-- Original:
CREATE INDEX idx_users_email ON users(email);

-- Auto-generated rollback:
DROP INDEX idx_users_email;
```

### Data Migration Detection

Detects hidden data transformations (UPDATE, INSERT...SELECT, DELETE) in migrations.

**Complexity Classification:**

| Complexity | Description | Base Speed |
|------------|-------------|------------|
| **SIMPLE_COMPUTATION** | String concat, math | 15,000 rows/sec |
| **MODERATE_LOGIC** | CASE statements, COALESCE | 10,000 rows/sec |
| **COMPLEX_JOINS** | Multi-table JOINs | 5,000 rows/sec |
| **BULK_DELETE** | Large DELETE operations | 20,000 rows/sec |

**Example Output:**

```
Data Migration Analysis:
─────────────────────────────────────
Operation: UPDATE
Complexity: SIMPLE_COMPUTATION
Estimated rows: 5.00M

Performance Estimate:
  Base speed: 15.00K rows/second
  Adjusted speed: 8.00K rows/second (3 indexes)
  Estimated duration: 10-12 minutes

Batching Recommendation:
  Should batch: Yes
  Recommended batch size: 10.00K
  Total batches: 500
  Allows cancellation: true

Table Bloat Impact:
  Estimated bloat: 16%
  Space reclaimable: 12.80 GB
  Vacuum required: Yes
  Recommendation: Run VACUUM ANALYZE users after migration to reclaim ~12 GB
```

**Batching recommendations:**

For large data migrations, TAPA suggests batched approaches:

```sql
-- Batched update example:
DO $$
DECLARE
  affected_rows INT;
BEGIN
  LOOP
    UPDATE users SET full_name = first_name || ' ' || last_name 
    WHERE full_name IS NULL LIMIT 10000;
    
    GET DIAGNOSTICS affected_rows = ROW_COUNT;
    EXIT WHEN affected_rows = 0;
    PERFORM pg_sleep(0.1); -- Pause between batches
  END LOOP;
END $$;
```

## JSON Output

All advanced features are included in JSON output:

```json
{
  "migrations": [
    {
      "operations": [
        {
          "sql": "ALTER TABLE users ALTER COLUMN email TYPE TEXT",
          "disk_space_analysis": {
            "current_state": {
              "total_size_bytes": 102005473280
            },
            "migration_requirements": {
              "requires_rewrite": true,
              "peak_disk_usage_bytes": 210453397504
            },
            "system_check": {
              "has_sufficient_space": false
            }
          },
          "rollback_analysis": {
            "category": "DATA LOSS",
            "reversibility_score": 25,
            "is_reversible": false
          },
          "data_migration_analysis": {
            "has_data_migration": true,
            "operation_type": "UPDATE",
            "complexity": "SIMPLE_COMPUTATION"
          }
        }
      ]
    }
  ]
}
```

## Performance Impact

Advanced features add minimal overhead:

- **Disk Space Analysis**: <10ms (calculations only)
- **Rollback Analysis**: <5ms (pattern matching)
- **Data Migration Detection**: <10ms (SQL parsing)

Total overhead: ~25ms per operation

## Best Practices

1. **Always use `--comprehensive` for production migrations** - catches issues that basic analysis misses

2. **Review rollback strategies before deployment** - ensure you have a recovery plan

3. **Batch large data migrations** - follow TAPA's batching recommendations to avoid long-running locks

4. **Check disk space before migration** - insufficient space causes database failure

5. **VACUUM after data migrations** - reclaim space from dead tuples (PostgreSQL)

## Coming Soon

### Dry-Run Simulation

Execute migrations against temporary database copies to catch runtime errors:

- Constraint violations
- Permission issues
- Type conversion failures
- Resource exhaustion

### Concurrency Impact Analysis

Predict impact on production workload:

- Lock queue buildup
- Throughput degradation
- Connection pool saturation
- Optimal execution window

## Troubleshooting

**Q: Advanced features not showing in output**

A: Ensure you're using the `--comprehensive` flag:

```bash
tapa analyze migrations/ --db $DATABASE_URL --comprehensive
```

**Q: Disk space analysis shows "insufficient space" but I have plenty**

A: TAPA uses conservative estimates. The actual space required may be less. Review the breakdown to understand the calculation.

**Q: Performance estimates seem off**

A: Estimates assume typical hardware (200 MB/s disk throughput). Adjust with config:

```yaml
# .tapa.yml
analysis:
  disk_throughput_mbps: 500  # Faster NVMe SSD
  rewrite_factor: 1.5        # Optimized rewrite
```

## See Also

- [Architecture Guide](architecture.md) - System design and algorithms
- [Batching Guide](batching-guide.md) - Risk-based deployment strategies
- [Configuration](../README.md#configuration) - Tuning parameters
