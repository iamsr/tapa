# Advanced Features Guide

TAPA includes five advanced features for comprehensive migration analysis. These features are enabled with the `--comprehensive` flag.

## Overview

| Feature | Description | Output |
|---------|-------------|--------|
| **Disk Space Requirements** | Calculates disk space needed before, during, and after migration | Space analysis with warnings |
| **Rollback Analysis** | Determines reversibility and generates rollback strategies | Rollback category and recovery steps |
| **Data Migration Detection** | Detects UPDATE/INSERT/DELETE operations in migrations | Performance estimates and batching advice |
| **Dry-Run Simulation** | Executes migrations in temporary isolated schemas | Runtime error detection before production |
| **Concurrency Impact Analysis** | Predicts impact on concurrent queries and suggests safer alternatives | Lock analysis, workload metrics, and optimal execution window |

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

## Dry-Run Simulation

Execute migrations in temporary isolated schemas to detect runtime errors before production deployment.

**Example Usage:**

```bash
tapa analyze migrations/ --db $DATABASE_URL --dry-run
```

**What it detects:**

- **Constraint Violations**: Foreign key, unique, check constraints
- **Syntax Errors**: Invalid SQL that parser missed
- **Type Conversion Failures**: Data type incompatibilities
- **Permission Issues**: Missing privileges
- **Resource Exhaustion**: Insufficient temp space

**Example Output:**

```
Dry-Run Execution:
─────────────────────────────────────
Status: FAILED
Execution time: 245 ms
Errors: 2
Warnings: 0
Temp schema: tapa_temp_1707734400
Rolled back: true

Errors:

  1. [CONSTRAINT_VIOLATION] foreign key constraint "fk_user" violation
     SQL: ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id)...
     Details: Key (user_id)=(999) is not present in table "users"

  2. [SYNTAX_ERROR] syntax error at or near "WHERE"
     SQL: SELECT FROM WHERE
     
✗ Migration would fail with 2 errors
```

**How it works:**

1. Creates temporary schema with unique name
2. Clones table structures (without data)
3. Executes migration SQL in transaction
4. Captures all errors with detailed context
5. Rolls back transaction (no permanent changes)
6. Drops temporary schema

**Limitations:**

- Only detects schema-level issues (not data-dependent)
- May not catch production-specific problems (load, concurrency)
- Requires database connection with CREATE SCHEMA privileges
- Performance estimates may differ from production

**Database-specific behavior:**

- **PostgreSQL**: Uses schemas, requires CREATE privilege
- **MySQL**: Uses databases, requires CREATE DATABASE privilege

## Concurrency Impact Analysis

Predict how migrations affect concurrent database operations and get recommendations for safer alternatives.

**Example Usage:**

```bash
# Analyze concurrency impact
tapa analyze migrations/ --db $DATABASE_URL --concurrency

# Include in comprehensive analysis
tapa analyze migrations/ --db $DATABASE_URL --comprehensive
```

**What it analyzes:**

- **Lock Types & Duration**: Predicts lock behavior and hold time
- **Blocked Queries**: Estimates number and types of queries affected
- **Current Workload**: Analyzes active connections and query patterns
- **Safer Alternatives**: Suggests concurrency-safe approaches
- **Downtime Estimation**: Predicts service disruption duration

**Example Output:**

```
Concurrency Impact:
─────────────────────────────────────
Impact Level: HIGH (score: 75/100)
Concurrency Safe: ✗ NO

Lock Details:
  Lock Type: ACCESS_EXCLUSIVE
  Duration: 30-60 seconds (45000 ms)
  Blocks: ALL operations (reads + writes)
  Estimated blocked queries: 150
  Lock acquisition risk: high

Current Workload:
  Active connections: 25
  Queries/second: 50.5
  Table access: high
  ⚠️  PEAK LOAD PERIOD
  Long-running queries: 3 (may delay lock)

Estimated Downtime: 45 seconds

Safer Alternatives:

  1. Add column without default, then backfill in batches
     Lock type: ACCESS_EXCLUSIVE
     Impact reduction: -80%
     Requires: Manual multi-step process
     Steps:
       - Step 1: ALTER TABLE ADD COLUMN (no default) - fast, brief lock
       - Step 2: UPDATE table SET column = value in batches - no table lock
       - Step 3: ALTER TABLE ALTER COLUMN SET DEFAULT - fast, brief lock
     Tradeoffs:
       • Requires manual batching logic
       • Column temporarily NULL during backfill
       • More complex deployment process

Recommendations:
  • ⚠️  HIGH IMPACT: This operation will block all queries for >30 seconds
  • Consider scheduling during maintenance window
  • ⚠️  Current workload is HIGH - consider delaying migration
  • Set statement_timeout to prevent indefinite blocking
  • Monitor active queries before executing: SELECT * FROM pg_stat_activity
```

**Impact Scoring:**

Concurrency impact is scored 0-100 based on:

1. **Lock Type** (0-40 points):
   - ACCESS EXCLUSIVE: 40 points (most restrictive)
   - EXCLUSIVE: 30 points
   - SHARE: 20 points
   - SHARE UPDATE EXCLUSIVE: 10 points (concurrent)
   - ROW EXCLUSIVE: 5 points (minimal impact)

2. **Lock Duration** (0-30 points):
   - >5 minutes: 30 points
   - >1 minute: 25 points
   - >30 seconds: 20 points
   - >10 seconds: 15 points
   - <10 seconds: 5-10 points

3. **Workload** (0-20 points):
   - Very high traffic: 20 points
   - High traffic: 15 points
   - Medium traffic: 10 points
   - Low traffic: 5 points

4. **Blocked Operations** (0-10 points):
   - Blocks reads + writes: 10 points
   - Blocks writes only: 5 points

**Impact Levels:**

- **MINIMAL** (0-20): Safe to run anytime
- **LOW** (21-40): Run during normal hours with monitoring
- **MEDIUM** (41-60): Run during low-traffic period
- **HIGH** (61-80): Coordinate with team, off-hours recommended
- **CRITICAL** (81-100): Maintenance window required

**Workload Analysis:**

When database connection is available, TAPA queries system tables to understand current load:

- **PostgreSQL**: Uses `pg_stat_activity`, `pg_stat_statements` (if available)
- **MySQL**: Uses `information_schema.processlist`, `performance_schema` (if enabled)

Workload metrics include:
- Active connections
- Queries per second to affected table
- Table access frequency
- Long-running queries (>5 seconds)
- Peak load detection

**Safer Alternatives:**

TAPA suggests concurrency-safe alternatives for common high-impact operations:

| Operation | Alternative | Impact Reduction |
|-----------|-------------|------------------|
| CREATE INDEX | CREATE INDEX CONCURRENTLY | 70% |
| ADD COLUMN with DEFAULT | Multi-step add + backfill | 80% |
| ALTER COLUMN type | New column + swap | 75% |
| DROP COLUMN | App change + delayed drop | 90% |

Each alternative includes:
- Step-by-step implementation guide
- Database version requirements
- Tradeoffs and considerations
- Estimated time impact

**Best Practices:**

1. **Run Analysis Before Migration**
   ```bash
   tapa analyze migration.sql --db $DATABASE_URL --concurrency
   ```

2. **Check During Off-Peak Hours**
   - Workload analysis is time-dependent
   - Re-run analysis during planned migration window

3. **Use Safer Alternatives for High Impact**
   - Score >60: Strongly consider alternatives
   - Score >80: Use alternatives or maintenance window

4. **Set Timeouts**
   ```sql
   SET statement_timeout = '60s';
   SET lock_timeout = '5s';
   ```

5. **Monitor Active Queries**
   ```sql
   -- PostgreSQL
   SELECT * FROM pg_stat_activity WHERE state = 'active';
   
   -- MySQL
   SHOW PROCESSLIST;
   ```

**Limitations:**

- Workload analysis requires database connection
- Estimates based on current load (may not reflect migration window)
- Cannot predict external factors (network issues, hardware failures)
- Some alternatives require manual implementation
- Lock acquisition can be delayed by existing queries

**Database Support:**

| Feature | PostgreSQL | MySQL |
|---------|------------|-------|
| Lock impact analysis | ✓ | ✓ |
| Workload analysis | ✓ | ✓ |
| Concurrent index creation | ✓ | Limited |
| Lock timeout | ✓ | ✓ |
| Statement timeout | ✓ | ✓ |

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
