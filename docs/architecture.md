# TAPA Architecture Documentation

**Version:** 0.3.0  
**Last Updated:** February 2026

## Table of Contents

1. [System Overview](#1-system-overview)
2. [Component Architecture](#2-component-architecture)
3. [Data Flow](#3-data-flow)
4. [Key Algorithms](#4-key-algorithms)
5. [Database Support](#5-database-support)
6. [Configuration](#6-configuration)
7. [Extension Points](#7-extension-points)
8. [Testing Architecture](#8-testing-architecture)
9. [Performance Considerations](#9-performance-considerations)
10. [Security](#10-security)
11. [Design Decisions](#11-design-decisions)

---

## 1. System Overview

### 1.1 Purpose

TAPA (Table Alteration Planning Assistant) is a static analysis tool that predicts the production impact of database migrations **before execution**. It analyzes SQL migration files and provides:

- **Lock detection** - Identifies lock types and durations
- **Risk scoring** - Calculates risk scores (0-100) with categorization
- **Time estimation** - Predicts execution time based on table size
- **Dependency analysis** - Finds breaking changes (indexes, views, foreign keys)
- **Safe alternatives** - Generates multi-step safer approaches
- **Migration batching** - Groups operations by risk for incremental deployment

### 1.2 Core Capabilities

```
┌─────────────────────────────────────────────────────────────┐
│                         TAPA System                          │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Parser     │  │   Analyzer   │  │  Introspector│      │
│  │              │  │              │  │              │      │
│  │ - PostgreSQL │  │ - Lock Det.  │  │ - Table Stats│      │
│  │ - MySQL      │  │ - Risk Calc. │  │ - Indexes    │      │
│  │              │  │ - Time Est.  │  │ - DB Queries │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                  │                  │              │
│         └──────────────────┼──────────────────┘              │
│                            │                                 │
│              ┌─────────────▼──────────────┐                  │
│              │     Core Data Models       │                  │
│              │  - Operation               │                  │
│              │  - Migration               │                  │
│              │  - AnalysisResult          │                  │
│              │  - BatchingStrategy        │                  │
│              └─────────────┬──────────────┘                  │
│                            │                                 │
│              ┌─────────────▼──────────────┐                  │
│              │     Output Formatters      │                  │
│              │  - Table (Human)           │                  │
│              │  - JSON (Machine)          │                  │
│              │  - YAML                    │                  │
│              └────────────────────────────┘                  │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### 1.3 Supported Databases

- **PostgreSQL** 9.6+ (via pg_query parser)
- **MySQL** 5.7+, 8.0+ (via vitess parser)

### 1.4 High-Level Architecture

```
┌─────────────┐
│   CLI App   │  cmd/tapa/main.go
└──────┬──────┘
       │
       ├──────► analyze command  (cmd/tapa/analyze.go)
       └──────► batch command    (cmd/tapa/batch.go)
                      │
       ┌──────────────┴───────────────┐
       │                              │
       ▼                              ▼
┌─────────────┐              ┌─────────────┐
│   Parser    │              │  Analyzer   │
│  (SQL AST)  │─────────────►│ (Risk/Time) │
└─────────────┘              └──────┬──────┘
                                    │
                             ┌──────┴──────┐
                             │             │
                             ▼             ▼
                      ┌──────────┐  ┌──────────┐
                      │Introspect│  │ Batcher  │
                      │(DB Conn) │  │(Grouping)│
                      └──────────┘  └──────────┘
```

---

## 2. Component Architecture

### 2.1 Parser Subsystem

**Location:** `internal/parser/`

The parser subsystem converts SQL migration files into structured `Operation` objects.

#### 2.1.1 Parser Interface

```go
// internal/parser/parser.go:12-18
type Parser interface {
    Parse(sql string) ([]*models.Operation, error)
    ParseFile(filePath string) (*models.Migration, error)
}
```

#### 2.1.2 PostgreSQL Parser

**Location:** `internal/parser/postgres/postgres.go`

Uses the `pg_query_go` library (libpg_query wrapper) for accurate PostgreSQL parsing.

**Key Operations:**
- `AlterTableStmt` → Detects ADD/DROP/ALTER COLUMN operations
- `CreateStmt` → CREATE TABLE detection
- `DropStmt` → DROP TABLE/INDEX detection
- `IndexStmt` → CREATE INDEX detection

**Example Parse Flow:**

```go
// internal/parser/postgres/postgres.go:64-80
func (p *Parser) parseStatement(stmt *pg_query.RawStmt, originalSQL string) (*models.Operation, error) {
    node := stmt.Stmt
    
    switch n := node.Node.(type) {
    case *pg_query.Node_AlterTableStmt:
        return p.parseAlterTable(n.AlterTableStmt, originalSQL)
    case *pg_query.Node_CreateStmt:
        return p.parseCreateTable(n.CreateStmt, originalSQL)
    // ...
    }
}
```

**Limitations:**
- Does not detect CONCURRENTLY keyword (analyzer handles this)
- Simple table name extraction (schema-qualified names supported)

#### 2.1.3 MySQL Parser

**Location:** `internal/parser/mysql/mysql.go`

Uses the Vitess SQL parser for MySQL DDL statements.

**Key Operations:**
- `AlterTable` with options:
  - `AddColumns` → ADD COLUMN
  - `DropColumn` → DROP COLUMN
  - `ModifyColumn` / `ChangeColumn` → ALTER COLUMN
  - `AddIndexDefinition` → CREATE INDEX
  - `DropKey` → DROP INDEX
- `CreateTable` → CREATE TABLE
- `DropTable` → DROP TABLE

**Algorithm/Lock Clause Detection:**

MySQL parser **does not** extract ALGORITHM or LOCK clauses. These are detected later in the analyzer phase (internal/analyzer/mysql/analyzer.go:116-134).

#### 2.1.4 Factory Pattern

```go
// internal/parser/parser.go:21-30
func GetParser(dbType string) (Parser, error) {
    switch dbType {
    case "postgresql":
        return postgres.NewParser(), nil
    case "mysql":
        return mysql.NewParser(), nil
    default:
        return nil, fmt.Errorf("unsupported database type: %s", dbType)
    }
}
```

---

### 2.2 Analyzer Subsystem

**Location:** `internal/analyzer/`

The analyzer is the heart of TAPA, enriching operations with risk scores, time estimates, lock types, and recommendations.

#### 2.2.1 Analyzer Interface

```go
// internal/analyzer/analyzer.go:14-17
type Analyzer interface {
    Analyze(ctx context.Context, op *models.Operation) error
}
```

#### 2.2.2 PostgreSQL Analyzer

**Location:** `internal/analyzer/postgres/analyzer.go`

##### Analysis Pipeline (6 Steps)

The analyzer follows a strict 6-step pipeline for each operation:

```go
// internal/analyzer/postgres/analyzer.go:48-86
func (a *Analyzer) Analyze(ctx context.Context, op *models.Operation) error {
    // Step 1: Detect lock type and requirements
    a.detectLockType(op)
    
    // Step 2: Get table stats (if needed)
    stats, err := a.introspector.GetTableStats(ctx, op.TableName)
    
    // Step 3: Estimate duration
    a.estimateDuration(op, stats)
    
    // Step 4: Calculate risk score
    a.calculateRiskScore(op, stats)
    
    // Step 5: Determine backward compatibility
    a.setBackwardCompatibility(op)
    
    // Step 6: Generate recommendations
    a.generateRecommendations(op, stats)
    
    return nil
}
```

##### Lock Detection

**Location:** `internal/analyzer/postgres/analyzer.go:90-182`

Maps operation types to PostgreSQL lock modes:

| Operation Type | Lock Type | Duration (ms) | Requires Rewrite |
|----------------|-----------|---------------|------------------|
| ADD COLUMN (with DEFAULT) | ACCESS EXCLUSIVE | 100 | Yes (volatile functions) |
| ADD COLUMN (no DEFAULT) | ACCESS EXCLUSIVE | 100 | No |
| DROP COLUMN | ACCESS EXCLUSIVE | 100 | No (instant in PG 11+) |
| ALTER COLUMN | ACCESS EXCLUSIVE | 1000 | Yes |
| CREATE INDEX | SHARE | 100 | No |
| CREATE INDEX CONCURRENTLY | SHARE UPDATE EXCLUSIVE | 50 | No |
| DROP INDEX | ACCESS EXCLUSIVE | 100 | No |
| DROP INDEX CONCURRENTLY | SHARE UPDATE EXCLUSIVE | 50 | No |
| CREATE TABLE | NONE | 0 | No |
| DROP TABLE | ACCESS EXCLUSIVE | 50 | No |

**Volatile Function Detection:**

```go
// internal/analyzer/postgres/analyzer.go:103-109
volatileFuncs := []string{"now()", "random()", "uuid_generate", "gen_random_uuid()"}
for _, fn := range volatileFuncs {
    if strings.Contains(sqlLower, fn) {
        op.RequiresRewrite = true
        break
    }
}
```

##### Time Estimation

**Location:** `internal/analyzer/postgres/analyzer.go:185-212`

**Formula for Table Rewrite:**

```
estimatedTime = (tableSizeMB / diskThroughputMBps) * rewriteFactor
```

**Formula for Index Build:**

```
indexBuildTime = (tableSizeMB / diskThroughputMBps) * 1.5
```

**Defaults:**
- `diskThroughputMBps`: 200 MB/s (config.go:44)
- `rewriteFactor`: 2.0 (config.go:45)

**Example:**

```
Table: 10 GB
Throughput: 200 MB/s
Rewrite Factor: 2.0

Base Time = 10240 MB / 200 MB/s = 51.2 seconds
Estimated Time = 51.2 * 2.0 = 102.4 seconds
```

#### 2.2.3 MySQL Analyzer

**Location:** `internal/analyzer/mysql/analyzer.go`

##### MySQL-Specific Lock Detection

MySQL analyzer detects ALGORITHM and LOCK clauses using regex:

```go
// internal/analyzer/mysql/analyzer.go:117-134
func (a *Analyzer) detectAlgorithm(sql string) string {
    re := regexp.MustCompile(`(?i)ALGORITHM\s*=\s*['"]?(DEFAULT|INPLACE|COPY|INSTANT)['"]?`)
    matches := re.FindStringSubmatch(sql)
    if len(matches) > 1 {
        return strings.ToUpper(matches[1])
    }
    return "DEFAULT"
}

func (a *Analyzer) detectLock(sql string) string {
    re := regexp.MustCompile(`(?i)LOCK\s*=\s*['"]?(DEFAULT|NONE|SHARED|EXCLUSIVE)['"]?`)
    // ...
}
```

##### Algorithm-Specific Behavior

| Operation | ALGORITHM | Lock Type | Requires Rewrite |
|-----------|-----------|-----------|------------------|
| ADD COLUMN (with DEFAULT) | COPY (MySQL 5.7) | EXCLUSIVE | Yes |
| ADD COLUMN (no DEFAULT) | INPLACE/INSTANT | NONE | No |
| DROP COLUMN | INPLACE | NONE | No |
| ALTER COLUMN | COPY | EXCLUSIVE | Yes |
| CREATE INDEX | INPLACE | NONE | No |
| DROP INDEX | INPLACE | NONE | No |

##### pt-online-schema-change Integration

**Location:** `internal/analyzer/mysql/pt_osc.go`

Automatic pt-osc command generation for high-risk operations:

```go
// internal/analyzer/mysql/pt_osc.go:11-34
func GeneratePtOscCommand(op *models.Operation, host, database string) string {
    if op.RiskScore < 50 {
        return ""
    }
    
    alterClause := extractAlterClause(op.SQL)
    cmd := fmt.Sprintf("pt-online-schema-change --alter \"%s\" "+
        "--host=%s --user=root D=%s,t=%s --execute",
        alterClause, host, database, op.TableName)
    
    return cmd
}
```

Example output:

```bash
pt-online-schema-change --alter "ADD COLUMN status VARCHAR(50) DEFAULT 'active'" \
  --host=localhost --user=root D=mydb,t=users --execute
```

#### 2.2.4 Phase 2 Modules (Comprehensive Analysis)

When `--comprehensive` flag is used, additional modules are invoked:

##### Dependency Analyzer

**Location:** `internal/analyzer/dependencies/analyzer.go`

Finds database objects that depend on the operation's target:

```go
// internal/analyzer/dependencies/analyzer.go:13-16
type DependencyAnalyzer interface {
    FindDependencies(ctx context.Context, op *models.Operation) ([]models.Dependency, error)
}
```

**Detected Dependencies:**
- Indexes on columns being dropped/altered
- Views referencing tables/columns
- Foreign keys (future work)
- Triggers (future work)

**Example:**

```sql
-- Operation: DROP COLUMN users.email
-- Dependencies found:
-- - Index: idx_users_email (BREAKS)
-- - View: active_users (BREAKS if SELECT * or email in view)
```

##### Time Estimator

**Location:** `internal/analyzer/estimator/estimator.go`

Provides detailed time breakdown:

```go
// pkg/models/time_breakdown.go:9-15
type TimeBreakdown struct {
    TableRewriteSeconds    float64
    IndexBuildSeconds      float64
    ConstraintCheckSeconds float64
    MetadataUpdateSeconds  float64
    TotalSeconds           float64
}
```

**Example Output:**

```
Time Breakdown:
  Table Rewrite: 120.5s
  Index Build: 45.2s
  Metadata: 0.1s
  Total: 165.8s
```

##### Alternative Generator

**Location:** `internal/analyzer/alternatives/generator.go`

Generates safer multi-step alternatives for high-risk operations.

**PostgreSQL Alternatives:**

1. **ADD COLUMN with DEFAULT** → 3-step approach
   ```sql
   -- Step 1: Add column without DEFAULT (fast)
   ALTER TABLE users ADD COLUMN status VARCHAR(50);
   
   -- Step 2: Backfill (offline job)
   UPDATE users SET status = 'active' WHERE status IS NULL;
   
   -- Step 3: Set DEFAULT (fast)
   ALTER TABLE users ALTER COLUMN status SET DEFAULT 'active';
   ```

2. **CREATE INDEX without CONCURRENTLY** → Use CONCURRENTLY
   ```sql
   CREATE INDEX CONCURRENTLY idx_users_email ON users(email);
   ```

3. **ALTER COLUMN TYPE** → Multi-column approach
   ```sql
   -- Step 1: Add new column
   ALTER TABLE users ADD COLUMN email_new TEXT;
   
   -- Step 2: Migrate data
   UPDATE users SET email_new = email;
   
   -- Step 3: Rename columns
   ALTER TABLE users RENAME COLUMN email TO email_old;
   ALTER TABLE users RENAME COLUMN email_new TO email;
   ```

**MySQL Alternatives:**

1. **ADD COLUMN with DEFAULT** → 3-step approach (similar to PostgreSQL)
2. **CREATE INDEX** → Suggest ALGORITHM=INPLACE, LOCK=NONE

---

### 2.3 Batcher Subsystem

**Location:** `internal/analyzer/batcher/batcher.go`

Groups operations by risk level for safer incremental deployment.

#### 2.3.1 Batching Strategy

```go
// internal/analyzer/batcher/batcher.go:35-122
func (b *postgresBatcher) GenerateBatches(ops []*models.Operation) (*models.BatchingStrategy, error) {
    // Group by risk level
    var lowRisk []*models.Operation     // score < 26
    var mediumRisk []*models.Operation  // 26 <= score < 51
    var highRisk []*models.Operation    // score >= 51
    
    // Batch 1: Low-risk (parallel)
    // Batch 2: Medium-risk (sequential, depends on Batch 1)
    // Batch 3+: One high-risk operation per batch
}
```

#### 2.3.2 Batching Rules

| Risk Level | Batching Strategy | Prerequisites | Can Run in Parallel |
|------------|-------------------|---------------|---------------------|
| LOW (0-25) | Group together | None | Yes |
| MEDIUM (26-50) | Group together | All previous batches | No |
| HIGH (51-75) | Isolate (one per batch) | All previous batches | No |
| CRITICAL (76-100) | Isolate (one per batch) | All previous batches | No |

**Example Output:**

```
Migration Batching Strategy
================================================================================

Summary:
  Total Operations: 5
  Total Batches: 4
  Estimated Total Time: 245.6s
  Max Risk Level: HIGH

Batch #1 (LOW):
  Operations: 2
  Risk Score: 20/100
  Estimated Time: 0.2s
  Parallel Execution: true
  Rationale: Low-risk operations can be deployed immediately
  
  Operations:
    1. CREATE_TABLE on audit_log (Risk: 0)
    2. CREATE_INDEX on users(email) CONCURRENTLY (Risk: 20)

Batch #2 (MEDIUM):
  Operations: 2
  Risk Score: 45/100
  Estimated Time: 5.4s
  Parallel Execution: false
  Prerequisites: Batches [1]
  Rationale: Medium-risk operations should be deployed during low-traffic periods
  
  Operations:
    1. ADD_COLUMN on users(phone) (Risk: 35)
    2. DROP_INDEX on users(old_idx) (Risk: 45)

Batch #3 (HIGH):
  Operations: 1
  Risk Score: 80/100
  Estimated Time: 240.0s
  Parallel Execution: false
  Prerequisites: Batches [1, 2]
  Rationale: High-risk operation requires maintenance window
  
  Operations:
    1. ALTER_COLUMN on users(email) TYPE TEXT (Risk: 80)
```

---

### 2.4 Introspector Subsystem

**Location:** `internal/db/`

Provides live database introspection for accurate analysis.

#### 2.4.1 Introspector Interface

```go
// internal/db/introspector.go:33-46
type Introspector interface {
    Connect(ctx context.Context) error
    Close() error
    GetTableStats(ctx context.Context, tableName string) (*TableStats, error)
    TableExists(ctx context.Context, tableName string) (bool, error)
}
```

#### 2.4.2 TableStats Structure

```go
// internal/db/introspector.go:8-14
type TableStats struct {
    TableName      string
    RowCount       int64
    TableSizeBytes int64
    IndexSizeBytes int64
    Indexes        []IndexInfo
}
```

#### 2.4.3 PostgreSQL Introspector

**Location:** `internal/db/postgres/introspector.go`

Queries:
- `pg_class` for table sizes
- `pg_stat_user_tables` for row counts
- `pg_indexes` for index information

**Example Query:**

```sql
SELECT 
    pg_total_relation_size(c.oid) as table_size_bytes,
    n_live_tup as row_count
FROM pg_class c
JOIN pg_stat_user_tables s ON c.relname = s.relname
WHERE c.relname = $1;
```

#### 2.4.4 MySQL Introspector

**Location:** `internal/db/mysql/introspector.go`

Queries:
- `information_schema.TABLES` for table statistics
- `information_schema.STATISTICS` for index information

**Example Query:**

```sql
SELECT 
    TABLE_ROWS as row_count,
    DATA_LENGTH as table_size_bytes,
    INDEX_LENGTH as index_size_bytes
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?;
```

#### 2.4.5 Fallback Behavior

When introspector is unavailable (dry-run mode):

```go
// internal/analyzer/postgres/analyzer.go:59-72
stats = &db.TableStats{
    TableName:      op.TableName,
    RowCount:       1000000,                 // Assume 1M rows
    TableSizeBytes: 10 * 1024 * 1024 * 1024, // Assume 10 GB
}
```

This provides conservative estimates without requiring database access.

---

### 2.5 Output Subsystem

**Location:** `internal/output/`

Formats analysis results for different audiences.

#### 2.5.1 Format Interface

```go
// internal/output/formatter.go:15-26
func Format(w io.Writer, result *models.AnalysisResult, format string) error {
    switch format {
    case "table":
        return FormatTable(w, result)
    case "json":
        return FormatJSON(w, result)
    case "yaml":
        return FormatYAML(w, result)
    }
}
```

#### 2.5.2 Table Formatter (Human-Readable)

**Features:**
- Color-coded risk levels (via `internal/output/colors.go`)
- ASCII table borders
- Summary cards (via `internal/ui/card.go`)
- Detailed recommendations

**Color Mapping:**

```go
// internal/output/colors.go (inferred)
Risk Score 0-25   → Green   (LOW)
Risk Score 26-50  → Blue    (MEDIUM)
Risk Score 51-75  → Yellow  (HIGH)
Risk Score 76-100 → Red     (CRITICAL)
```

**Example Output:**

```
┌────────────────────────────────────────────────────────┐
│ Migration Summary: 001_add_column.sql                  │
├────────────────────────────────────────────────────────┤
│ Operations: 1                                          │
│ Max Risk: 🟡 HIGH (Risk: 75)                          │
│ Estimated Time: 120.5s                                 │
└────────────────────────────────────────────────────────┘

┌────────────────────┬─────────────────┬──────────────────────────────┐
│ OPERATION          │ TABLE           │ DETAILS                      │
├────────────────────┼─────────────────┼──────────────────────────────┤
│ ADD_COLUMN         │ users           │ Rewrite, Exclusive Lock, ... │
└────────────────────┴─────────────────┴──────────────────────────────┘
```

#### 2.5.3 JSON Formatter (Machine-Readable)

Used for CI/CD integration and automation:

```json
{
  "migrations": [
    {
      "file_path": "001_add_column.sql",
      "operations": [
        {
          "sql": "ALTER TABLE users ADD COLUMN status VARCHAR(50) DEFAULT 'active'",
          "type": "ADD_COLUMN",
          "table_name": "users",
          "lock_type": "ACCESS_EXCLUSIVE",
          "lock_duration_ms": 100,
          "requires_rewrite": true,
          "estimated_time_seconds": 120.5,
          "risk_score": 75,
          "backward_compatible": true,
          "recommendations": [
            "Add column without DEFAULT first, then set DEFAULT separately to avoid table rewrite"
          ]
        }
      ]
    }
  ],
  "database_type": "postgresql",
  "fail_on_risk_level": "HIGH"
}
```

#### 2.5.4 YAML Formatter

Similar to JSON but in YAML format for better human readability.

---

### 2.6 UI Components

**Location:** `internal/ui/`

Provides reusable UI components for terminal output.

#### 2.6.1 Progress Bar

**Location:** `internal/ui/progress.go`

Shows parsing progress when analyzing multiple files:

```go
// internal/ui/progress.go (inferred structure)
type ProgressBar struct {
    total   int
    current int
    message string
}

func (p *ProgressBar) Increment()
func (p *ProgressBar) Finish()
```

**Example:**

```
Parsing files: [████████████████████] 10/10 files
```

#### 2.6.2 Summary Card

**Location:** `internal/ui/card.go`

Displays migration summary at the top of output:

```go
// internal/ui/card.go (inferred)
func FormatSummaryCard(migration *models.Migration) string
```

---

## 3. Data Flow

### 3.1 Analyze Command Flow

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. CLI Entry Point: cmd/tapa/analyze.go                         │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Load Configuration: internal/config/config.go                │
│    - Read .tapa.yml (if present)                                │
│    - Apply CLI flags (override config)                          │
│    - Validate configuration                                     │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Initialize Components                                        │
│    - Get parser (PostgreSQL/MySQL)                              │
│    - Get introspector (optional, connect to DB)                 │
│    - Get analyzer (with throughput/rewrite config)              │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. Find Migration Files: cmd/tapa/analyze.go:260-284            │
│    - If file: return [file]                                     │
│    - If directory: walk and find *.sql files                    │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 5. Parse Each File: Parser.ParseFile()                          │
│    ┌─────────────────────────────────────────┐                 │
│    │ PostgreSQL:                             │                 │
│    │  - pg_query.Parse()                     │                 │
│    │  - Walk AST nodes                       │                 │
│    │  - Create Operation objects             │                 │
│    │                                         │                 │
│    │ MySQL:                                  │                 │
│    │  - sqlparser.Parse()                    │                 │
│    │  - Detect AlterTable/CreateTable/etc    │                 │
│    │  - Create Operation objects             │                 │
│    └─────────────────────────────────────────┘                 │
│                                                                  │
│    Output: Migration { FilePath, Operations[] }                 │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 6. Analyze Each Operation: Analyzer.Analyze()                   │
│    ┌─────────────────────────────────────────┐                 │
│    │ Step 1: Detect lock type                │                 │
│    │  - Map operation type → lock type       │                 │
│    │  - Detect CONCURRENTLY (PG)             │                 │
│    │  - Detect ALGORITHM/LOCK (MySQL)        │                 │
│    │                                         │                 │
│    │ Step 2: Get table stats (if available)  │                 │
│    │  - Query introspector                   │                 │
│    │  - Fallback to conservative estimates   │                 │
│    │                                         │                 │
│    │ Step 3: Estimate duration                │                 │
│    │  - Calculate rewrite time               │                 │
│    │  - Calculate index build time           │                 │
│    │                                         │                 │
│    │ Step 4: Calculate risk score            │                 │
│    │  - baseLockScore + tableSizeScore +     │                 │
│    │    durationScore                        │                 │
│    │                                         │                 │
│    │ Step 5: Determine backward compat       │                 │
│    │  - Drop operations → not compatible     │                 │
│    │  - Add nullable → compatible            │                 │
│    │                                         │                 │
│    │ Step 6: Generate recommendations        │                 │
│    │  - Operation-specific advice            │                 │
│    │  - Risk-based recommendations           │                 │
│    └─────────────────────────────────────────┘                 │
│                                                                  │
│    Output: Operation { ..., RiskScore, Recommendations }        │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼ (if --comprehensive)
┌─────────────────────────────────────────────────────────────────┐
│ 7. Phase 2 Enhancements (Optional)                              │
│    ┌─────────────────────────────────────────┐                 │
│    │ A. Dependency Analysis                  │                 │
│    │    - Find indexes on column             │                 │
│    │    - Find views referencing table       │                 │
│    │    - Add to op.Dependencies[]           │                 │
│    │                                         │                 │
│    │ B. Time Breakdown                       │                 │
│    │    - Calculate TableRewriteSeconds      │                 │
│    │    - Calculate IndexBuildSeconds        │                 │
│    │    - Add to op.TimeBreakdown            │                 │
│    │                                         │                 │
│    │ C. Alternative Strategies               │                 │
│    │    - Check if alternatives apply        │                 │
│    │    - Generate multi-step strategies     │                 │
│    │    - Add to op.Alternatives[]           │                 │
│    └─────────────────────────────────────────┘                 │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 8. Build AnalysisResult                                         │
│    - Collect all migrations                                     │
│    - Collect errors                                             │
│    - Set database type                                          │
│    - Set fail threshold                                         │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 9. Format Output: output.Format()                               │
│    - Select formatter (table/json/yaml)                         │
│    - Write to stdout                                            │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 10. Check Failures                                              │
│     - Compare max risk score vs. threshold                      │
│     - Exit with error code if exceeded                          │
└─────────────────────────────────────────────────────────────────┘
```

**Key Files:**
- Entry point: `cmd/tapa/analyze.go:62-242`
- Parser factory: `internal/parser/parser.go:21-30`
- Analyzer factory: `internal/analyzer/analyzer.go:20-29`
- Output formatter: `internal/output/formatter.go:15-63`

### 3.2 Batch Command Flow

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. CLI Entry Point: cmd/tapa/batch.go                           │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Load Configuration (same as analyze)                         │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Initialize Parser, Introspector, Analyzer                    │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. Parse All Files → Collect All Operations                     │
│    - Loop through files                                         │
│    - Parse each file                                            │
│    - Analyze each operation                                     │
│    - Append to allOperations[]                                  │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 5. Generate Batching Strategy                                   │
│    analyzer.BatchOperations(allOperations)                      │
│    ┌─────────────────────────────────────────┐                 │
│    │ A. Group by risk level                  │                 │
│    │    - lowRisk (0-25)                     │                 │
│    │    - mediumRisk (26-50)                 │                 │
│    │    - highRisk (51+)                     │                 │
│    │                                         │                 │
│    │ B. Create Batch 1: Low-risk (parallel) │                 │
│    │    - CanRunInParallel: true             │                 │
│    │    - Prerequisites: []                  │                 │
│    │                                         │                 │
│    │ C. Create Batch 2: Medium-risk (seq)   │                 │
│    │    - CanRunInParallel: false            │                 │
│    │    - Prerequisites: [1]                 │                 │
│    │                                         │                 │
│    │ D. Create Batches 3+: One high-risk op │                 │
│    │    - CanRunInParallel: false            │                 │
│    │    - Prerequisites: [all previous]      │                 │
│    │                                         │                 │
│    │ E. Calculate metrics for each batch     │                 │
│    │    - MaxRiskScore                       │                 │
│    │    - TotalTimeSeconds                   │                 │
│    │    - RiskLevel                          │                 │
│    └─────────────────────────────────────────┘                 │
│                                                                  │
│    Output: BatchingStrategy { Batches[], TotalBatches, ... }    │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 6. Build BatchResult                                            │
│    - Wrap strategy in result object                             │
│    - Add database type                                          │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│ 7. Format Output: output.FormatBatching()                       │
│    - Select formatter (table/json/yaml)                         │
│    - Display batches with prerequisites                         │
│    - Show recommendations                                       │
└─────────────────────────────────────────────────────────────────┘
```

**Key Files:**
- Entry point: `cmd/tapa/batch.go:52-173`
- Batcher: `internal/analyzer/batcher/batcher.go:35-141`
- Output: `internal/output/formatter.go:170-211`

---

## 4. Key Algorithms

### 4.1 Risk Scoring Formula

**Location:** `internal/analyzer/postgres/analyzer.go:214-255`

The risk score is calculated as the sum of three components:

```
riskScore = baseLockScore + tableSizeScore + estimatedDurationScore
```

#### 4.1.1 Base Lock Score (0-40 points)

Maps lock types to base risk:

| Lock Type | Base Score | Rationale |
|-----------|------------|-----------|
| ACCESS EXCLUSIVE | 40 | Blocks all operations (reads + writes) |
| SHARE | 20 | Blocks writes, allows reads |
| SHARE UPDATE EXCLUSIVE | 10 | Minimal blocking |
| NONE | 0 | No blocking |

```go
// internal/analyzer/postgres/analyzer.go:222-234
baseLockScore := 0
switch op.LockType {
case models.LockTypeAccessExclusive:
    baseLockScore = 40
case models.LockTypeShare:
    baseLockScore = 20
case models.LockTypeShareUpdateExclusive:
    baseLockScore = 10
case models.LockTypeNone:
    baseLockScore = 0
default:
    baseLockScore = 10
}
```

#### 4.1.2 Table Size Score (0-30 points)

Scales with table size, capped at 30 points:

```
tableSizeScore = min((tableSizeGB / 10 GB) * 30, 30)
```

**Examples:**

| Table Size | Calculation | Score |
|------------|-------------|-------|
| 1 GB | (1/10) * 30 | 3 |
| 5 GB | (5/10) * 30 | 15 |
| 10 GB | (10/10) * 30 | 30 |
| 50 GB | (50/10) * 30 → cap | 30 |

```go
// internal/analyzer/postgres/analyzer.go:236-240
tableSizeScore := 0
if stats != nil && stats.TableSizeBytes > 0 {
    tableSizeGB := float64(stats.TableSizeBytes) / (1024 * 1024 * 1024)
    tableSizeScore = int(math.Min((tableSizeGB/10.0)*30, 30))
}
```

#### 4.1.3 Duration Score (0-30 points)

Scales with estimated time, capped at 30 points:

```
durationScore = min((estimatedMinutes / 60 minutes) * 30, 30)
```

**Examples:**

| Estimated Time | Calculation | Score |
|----------------|-------------|-------|
| 5 minutes | (5/60) * 30 | 2.5 → 2 |
| 30 minutes | (30/60) * 30 | 15 |
| 60 minutes | (60/60) * 30 | 30 |
| 120 minutes | (120/60) * 30 → cap | 30 |

```go
// internal/analyzer/postgres/analyzer.go:242-244
durationScore := 0
estimatedMinutes := op.EstimatedTimeSeconds / 60.0
durationScore = int(math.Min((estimatedMinutes/60.0)*30, 30))
```

#### 4.1.4 Final Risk Score

```go
// internal/analyzer/postgres/analyzer.go:246-254
op.RiskScore = baseLockScore + tableSizeScore + durationScore

// Ensure risk score is in valid range [0, 100]
if op.RiskScore > 100 {
    op.RiskScore = 100
}
if op.RiskScore < 0 {
    op.RiskScore = 0
}
```

#### 4.1.5 Risk Level Mapping

**Location:** `pkg/models/operation.go:67-78`

```go
func (o *Operation) RiskLevel() RiskLevel {
    switch {
    case o.RiskScore >= 76:
        return RiskLevelCritical  // 76-100
    case o.RiskScore >= 51:
        return RiskLevelHigh      // 51-75
    case o.RiskScore >= 26:
        return RiskLevelMedium    // 26-50
    default:
        return RiskLevelLow       // 0-25
    }
}
```

#### 4.1.6 Real-World Examples

**Example 1: Add nullable column (PostgreSQL)**

```sql
ALTER TABLE users ADD COLUMN phone VARCHAR(20);
```

- Base Lock Score: 40 (ACCESS EXCLUSIVE)
- Table Size Score: 0 (no rewrite, size irrelevant)
- Duration Score: 0 (< 1 second)
- **Total Risk Score: 40 → MEDIUM**

**Example 2: Add column with DEFAULT (PostgreSQL)**

```sql
ALTER TABLE users ADD COLUMN status VARCHAR(50) DEFAULT 'active';
```

Table: 10 GB, 10M rows  
Estimated time: 120 seconds

- Base Lock Score: 40 (ACCESS EXCLUSIVE)
- Table Size Score: 30 (10 GB → max)
- Duration Score: 15 ((2 min / 60 min) * 30)
- **Total Risk Score: 85 → CRITICAL**

**Example 3: Create index concurrently (PostgreSQL)**

```sql
CREATE INDEX CONCURRENTLY idx_users_email ON users(email);
```

Table: 5 GB  
Estimated time: 60 seconds

- Base Lock Score: 10 (SHARE UPDATE EXCLUSIVE)
- Table Size Score: 15 ((5/10) * 30)
- Duration Score: 15 ((1 min / 60 min) * 30)
- **Total Risk Score: 40 → MEDIUM**

### 4.2 MySQL Risk Scoring

**Location:** `internal/analyzer/mysql/analyzer.go:300-343`

MySQL uses a slightly different formula:

```
riskScore = lockScore + rewriteScore + durationScore + compatibilityScore
```

| Component | Max Points | Formula |
|-----------|------------|---------|
| Lock Type | 40 | EXCLUSIVE=40, SHARE=20, NONE=0 |
| Requires Rewrite | 30 | Yes=30, No=0 |
| Duration | 20 | (minutes/60) * 20, capped at 20 |
| Backward Compatibility | 10 | Not compatible=10, Compatible=0 |

This gives MySQL operations slightly different risk profiles than PostgreSQL.

---

### 4.3 Time Estimation Algorithm

**Location:** `internal/analyzer/postgres/analyzer.go:185-212`

#### 4.3.1 Table Rewrite Time

```
baseTimeSeconds = tableSizeMB / diskThroughputMBps
estimatedTime = baseTimeSeconds * rewriteFactor
```

**Default Configuration:**
- `diskThroughputMBps`: 200 MB/s (SSD)
- `rewriteFactor`: 2.0 (accounts for read + write + overhead)

**Example Calculation:**

```
Table: 10 GB = 10240 MB
Throughput: 200 MB/s
Rewrite Factor: 2.0

Base Time = 10240 / 200 = 51.2 seconds
Estimated Time = 51.2 * 2.0 = 102.4 seconds
```

#### 4.3.2 Index Build Time

```
tableSizeMB = stats.TableSizeBytes / (1024 * 1024)
indexBuildTime = (tableSizeMB / diskThroughputMBps) * 1.5
```

The 1.5x multiplier accounts for index creation being slower than raw I/O.

**For CONCURRENTLY:**
- Lock duration: 50ms (minimal)
- Total time: Full index build time

**For non-CONCURRENTLY (SHARE lock):**
- Lock duration: Full index build time
- Total time: Same as lock duration

```go
// internal/analyzer/postgres/analyzer.go:194-207
if op.Type == models.OperationTypeCreateIndex && stats != nil {
    tableSizeMB := float64(stats.TableSizeBytes) / (1024 * 1024)
    indexBuildTime := tableSizeMB / float64(a.diskThroughputMBps) * 1.5
    
    op.EstimatedTimeSeconds = indexBuildTime
    
    if op.LockType == models.LockTypeShareUpdateExclusive {
        op.LockDurationMS = 50  // CONCURRENTLY: minimal lock
    } else {
        op.LockDurationMS = int64(indexBuildTime * 1000)  // SHARE: full lock
    }
}
```

#### 4.3.3 Metadata Operations

Fast operations (no rewrite/index build):

```go
// internal/analyzer/postgres/analyzer.go:209-211
op.EstimatedTimeSeconds = float64(op.LockDurationMS) / 1000.0
```

Examples:
- DROP COLUMN (PG 11+): 100ms
- CREATE TABLE: 0ms
- DROP TABLE: 50ms

---

### 4.4 Backward Compatibility Detection

**Location:** `internal/analyzer/postgres/analyzer.go:258-291`

Rules:

| Operation | Condition | Backward Compatible? |
|-----------|-----------|----------------------|
| DROP COLUMN | Always | ❌ No |
| DROP TABLE | Always | ❌ No |
| DROP INDEX | Always | ❌ No |
| ALTER COLUMN | Type change | ❌ No |
| ADD COLUMN | NOT NULL without DEFAULT | ❌ No |
| ADD COLUMN | Nullable or with DEFAULT | ✅ Yes |
| CREATE INDEX | Always | ✅ Yes |
| CREATE TABLE | Always | ✅ Yes |

**Example:**

```sql
-- Not backward compatible
ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL;

-- Backward compatible
ALTER TABLE users ADD COLUMN email VARCHAR(255);
```

```go
// internal/analyzer/postgres/analyzer.go:269-277
case models.OperationTypeAddColumn:
    sqlLower := strings.ToLower(op.SQL)
    if strings.Contains(sqlLower, "not null") && !strings.Contains(sqlLower, "default") {
        op.BackwardCompatible = false  // NOT NULL without DEFAULT
    } else {
        op.BackwardCompatible = true
    }
```

---

### 4.5 Lock Detection Algorithm (PostgreSQL)

**Location:** `internal/analyzer/postgres/analyzer.go:90-182`

#### 4.5.1 Operation Type Mapping

Direct mapping for most operations:

```go
switch op.Type {
case models.OperationTypeAddColumn:
    op.LockType = models.LockTypeAccessExclusive
    op.LockDurationMS = 100
    
case models.OperationTypeDropColumn:
    op.LockType = models.LockTypeAccessExclusive
    op.LockDurationMS = 100
    op.RequiresRewrite = false  // Instant in PG 11+
    
case models.OperationTypeAlterColumn:
    op.LockType = models.LockTypeAccessExclusive
    op.RequiresRewrite = true
    op.LockDurationMS = 1000
}
```

#### 4.5.2 CREATE INDEX Detection

Uses SQL string analysis to detect CONCURRENTLY:

```go
// internal/analyzer/postgres/analyzer.go:147-156
case models.OperationTypeCreateIndex:
    if strings.Contains(sqlUpper, "CONCURRENTLY") {
        op.LockType = models.LockTypeShareUpdateExclusive
        op.LockDurationMS = 50
    } else {
        op.LockType = models.LockTypeShare
        op.LockDurationMS = 100
    }
```

#### 4.5.3 ADD COLUMN with DEFAULT Detection

Volatile function detection:

```go
// internal/analyzer/postgres/analyzer.go:103-118
if strings.Contains(sqlLower, "default") {
    volatileFuncs := []string{"now()", "random()", "uuid_generate", "gen_random_uuid()"}
    for _, fn := range volatileFuncs {
        if strings.Contains(sqlLower, fn) {
            op.RequiresRewrite = true
            break
        }
    }
    
    // Conservative: mark all DEFAULTs as requiring rewrite
    if !op.RequiresRewrite && strings.Contains(sqlLower, "default") {
        op.RequiresRewrite = true
    }
}
```

**Note:** PostgreSQL 11+ can add columns with constant DEFAULT values without rewriting the table. TAPA takes a conservative approach and assumes rewrite is needed.

---

### 4.6 Lock Detection Algorithm (MySQL)

**Location:** `internal/analyzer/mysql/analyzer.go:51-114`

#### 4.6.1 Two-Phase Detection

**Phase 1: Extract ALGORITHM and LOCK clauses**

```go
// internal/analyzer/mysql/analyzer.go:53-54
algorithm := a.detectAlgorithm(op.SQL)
lockClause := a.detectLock(op.SQL)
```

Regex patterns:
```go
ALGORITHM\s*=\s*['"]?(DEFAULT|INPLACE|COPY|INSTANT)['"]?
LOCK\s*=\s*['"]?(DEFAULT|NONE|SHARED|EXCLUSIVE)['"]?
```

**Phase 2: Route to operation-specific analyzer**

```go
// internal/analyzer/mysql/analyzer.go:57-72
switch op.Type {
case models.OperationTypeAddColumn:
    a.analyzeAddColumn(op, algorithm, lockClause)
case models.OperationTypeDropColumn:
    a.analyzeDropColumn(op, algorithm, lockClause)
// ...
}
```

#### 4.6.2 ADD COLUMN Analysis

```go
// internal/analyzer/mysql/analyzer.go:136-167
func (a *Analyzer) analyzeAddColumn(op *models.Operation, algorithm, lockClause string) {
    hasDefault := regexp.MustCompile(`\bdefault\s`).MatchString(sqlLower)
    
    if hasDefault {
        // ADD COLUMN with DEFAULT requires COPY in MySQL 5.7
        op.LockType = models.LockTypeExclusive
        op.RequiresRewrite = true
        op.LockDurationMS = 1000
    } else {
        if algorithm == "COPY" {
            op.LockType = models.LockTypeExclusive
            op.RequiresRewrite = true
        } else {
            // INPLACE or INSTANT
            op.LockType = models.LockTypeNone
            op.RequiresRewrite = false
            op.LockDurationMS = 50
        }
    }
    
    // Override with explicit LOCK clause
    if lockClause != "DEFAULT" {
        op.LockType = a.convertLockType(lockClause)
    }
}
```

**MySQL 5.7 vs 8.0 Differences:**

| Operation | MySQL 5.7 | MySQL 8.0 |
|-----------|-----------|-----------|
| ADD COLUMN (no DEFAULT) | INPLACE | INSTANT |
| ADD COLUMN (with DEFAULT) | COPY | INSTANT (8.0.12+) |

TAPA assumes MySQL 5.7 behavior for safety.

---

## 5. Database Support

### 5.1 PostgreSQL Version Support

**Supported Versions:** 9.6, 10, 11, 12, 13, 14, 15, 16

#### 5.1.1 Version-Specific Features

| Feature | PostgreSQL Version | TAPA Handling |
|---------|-------------------|---------------|
| `CREATE INDEX CONCURRENTLY` | 8.2+ | Detected, lowers risk score |
| `DROP INDEX CONCURRENTLY` | 9.2+ | Detected, lowers risk score |
| Instant ADD COLUMN with constant DEFAULT | 11+ | Conservative: assumes rewrite |
| Instant DROP COLUMN | 11+ | Marked as instant (RequiresRewrite=false) |
| Partitioned tables | 10+ | Not yet analyzed (future work) |

#### 5.1.2 Lock Type Matrix

| PostgreSQL Lock Mode | TAPA Constant | Blocks | Used For |
|----------------------|---------------|--------|----------|
| ACCESS EXCLUSIVE | `LockTypeAccessExclusive` | All operations | ALTER TABLE, DROP TABLE |
| SHARE | `LockTypeShare` | Writes | CREATE INDEX (non-concurrent) |
| SHARE UPDATE EXCLUSIVE | `LockTypeShareUpdateExclusive` | DDL | CREATE INDEX CONCURRENTLY |
| ROW EXCLUSIVE | `LockTypeRowExclusive` | DDL + SHARE | (not used by migrations) |
| EXCLUSIVE | `LockTypeExclusive` | Reads (except ACCESS SHARE) | (not used by migrations) |

#### 5.1.3 Connection String Format

```
postgresql://[user[:password]@][host][:port][/dbname][?param1=value1&...]
postgres://[user[:password]@][host][:port][/dbname][?param1=value1&...]
```

**Examples:**

```bash
# Local connection
--db postgres://localhost/mydb

# With credentials
--db postgresql://user:pass@localhost:5432/mydb

# SSL mode
--db "postgres://user:pass@host/db?sslmode=require"
```

#### 5.1.4 Required Permissions

For introspection:

```sql
-- Table statistics
SELECT FROM pg_class, pg_stat_user_tables;

-- Index information
SELECT FROM pg_indexes;

-- Must have CONNECT privilege on database
GRANT CONNECT ON DATABASE mydb TO tapa_user;
```

---

### 5.2 MySQL Version Support

**Supported Versions:** 5.7, 8.0

#### 5.2.1 Version-Specific Features

| Feature | MySQL 5.7 | MySQL 8.0 | TAPA Handling |
|---------|-----------|-----------|---------------|
| Online DDL (INPLACE) | ✅ Yes | ✅ Yes | Detected via ALGORITHM clause |
| Instant ADD COLUMN | ❌ No | ✅ Yes (8.0.12+) | Assumes 5.7 behavior |
| Instant DROP COLUMN | ❌ No | ✅ Yes (8.0.29+) | Assumes 5.7 behavior |
| Rename column without rebuild | ❌ No | ✅ Yes | Detected as ALGORITHM=INPLACE |

#### 5.2.2 ALGORITHM Support Matrix

| Operation | ALGORITHM | Lock | Requires Rewrite | TAPA Detection |
|-----------|-----------|------|------------------|----------------|
| ADD COLUMN (no DEFAULT) | INPLACE/INSTANT | NONE | No | ✅ Regex detection |
| ADD COLUMN (with DEFAULT) | COPY (5.7) / INSTANT (8.0+) | EXCLUSIVE (5.7) | Yes (5.7) | ✅ DEFAULT detection |
| DROP COLUMN | INPLACE | NONE | No | ✅ Regex detection |
| ALTER COLUMN TYPE | COPY | EXCLUSIVE | Yes | ✅ Marked as rewrite |
| CREATE INDEX | INPLACE | NONE | No | ✅ Regex detection |
| DROP INDEX | INPLACE | NONE | No | ✅ Regex detection |

#### 5.2.3 LOCK Clause Support

| LOCK Value | Effect | TAPA Mapping |
|------------|--------|--------------|
| `NONE` | No locks, allows reads + writes | `LockTypeNone` |
| `SHARED` | Allows reads, blocks writes | `LockTypeShare` |
| `EXCLUSIVE` | Blocks all operations | `LockTypeExclusive` |
| `DEFAULT` | Let MySQL decide | Determined by ALGORITHM |

#### 5.2.4 Connection String Format

```
mysql://[user[:password]@][protocol([host][:port])]/dbname[?param1=value1&...]
```

**Examples:**

```bash
# Local connection
--db mysql://root@localhost/mydb

# With password
--db mysql://user:pass@tcp(localhost:3306)/mydb

# Unix socket
--db mysql://user:pass@unix(/tmp/mysql.sock)/mydb
```

#### 5.2.5 Required Permissions

For introspection:

```sql
-- Table statistics
SELECT FROM information_schema.TABLES;

-- Index information
SELECT FROM information_schema.STATISTICS;

-- Column information
SELECT FROM information_schema.COLUMNS;

GRANT SELECT ON information_schema.* TO tapa_user;
```

#### 5.2.6 pt-online-schema-change Integration

**Location:** `internal/analyzer/mysql/pt_osc.go`

TAPA generates pt-osc commands for high-risk operations:

**Criteria for pt-osc recommendation:**
- Risk score ≥ 50
- Requires table rewrite
- Operation type: ADD/DROP/ALTER COLUMN

**Generated Command Format:**

```bash
pt-online-schema-change \
  --alter "<alter-clause>" \
  --host=<host> \
  --user=<user> \
  D=<database>,t=<table> \
  --execute
```

**Example:**

```sql
-- Original SQL
ALTER TABLE users ADD COLUMN status VARCHAR(50) DEFAULT 'active';

-- pt-osc command (in recommendations)
pt-online-schema-change \
  --alter "ADD COLUMN status VARCHAR(50) DEFAULT 'active'" \
  --host=localhost \
  --user=root \
  D=mydb,t=users \
  --execute
```

---

### 5.3 Database Feature Matrix

| Feature | PostgreSQL | MySQL | Implementation Location |
|---------|------------|-------|-------------------------|
| SQL Parsing | pg_query (libpg_query) | Vitess sqlparser | `internal/parser/` |
| Lock Detection | Pattern matching on op type | ALGORITHM/LOCK clause regex | `internal/analyzer/*/analyzer.go` |
| Online DDL | CONCURRENTLY keyword | ALGORITHM=INPLACE | Detected in analyzer |
| Table Stats | `pg_class`, `pg_stat_user_tables` | `information_schema.TABLES` | `internal/db/*/introspector.go` |
| Index Introspection | `pg_indexes` | `information_schema.STATISTICS` | `internal/db/*/introspector.go` |
| Dependency Detection | `pg_depend` (future) | `information_schema` (future) | `internal/analyzer/dependencies/` |
| External Tools | N/A | pt-online-schema-change | `internal/analyzer/mysql/pt_osc.go` |

---

## 6. Configuration

### 6.1 Configuration File Format

**Location:** `.tapa.yml` (YAML format)

**Example Configuration:**

```yaml
# .tapa.example.yml
database:
  type: postgresql  # or "mysql"
  url: "postgres://user:pass@localhost:5432/mydb"

analysis:
  disk_throughput_mbps: 200  # SSD throughput (MB/s)
  rewrite_factor: 2.0        # Table rewrite overhead multiplier
  fail_on_risk_level: ""     # "low", "medium", "high", or "critical"

output:
  format: table              # "table", "json", or "yaml"
  verbose: false
```

### 6.2 Configuration Parameters

#### 6.2.1 Database Configuration

**Location:** `internal/config/config.go:18-21`

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `database.type` | string | `""` | Database type (`postgresql`, `mysql`) |
| `database.url` | string | `""` | Database connection URL |

**Validation:**

```go
// internal/config/config.go:91-94
validDatabaseTypes := []string{"postgresql", "mysql"}
if !contains(validDatabaseTypes, c.Database.Type) {
    return fmt.Errorf("unsupported database type '%s'", c.Database.Type)
}
```

#### 6.2.2 Analysis Configuration

**Location:** `internal/config/config.go:24-28`

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `analysis.disk_throughput_mbps` | int | `200` | Disk throughput in MB/s for time estimation |
| `analysis.rewrite_factor` | float64 | `2.0` | Multiplier for table rewrite time |
| `analysis.fail_on_risk_level` | string | `""` | Exit with error if risk exceeds threshold |

**Disk Throughput Presets:**

| Storage Type | Recommended Value |
|--------------|-------------------|
| HDD (7200 RPM) | 100 MB/s |
| SATA SSD | 200-300 MB/s |
| NVMe SSD | 500-1000 MB/s |
| Cloud (AWS EBS gp3) | 125-250 MB/s |
| Cloud (AWS EBS io2) | 500-1000 MB/s |

**Rewrite Factor:**
- **Conservative (default):** 2.0
- **Optimistic (fast hardware):** 1.5
- **Pessimistic (slow hardware):** 3.0

**Validation:**

```go
// internal/config/config.go:113-121
if c.Analysis.DiskThroughputMBps <= 0 {
    return fmt.Errorf("disk_throughput_mbps must be positive")
}

if c.Analysis.RewriteFactor <= 0 {
    return fmt.Errorf("rewrite_factor must be positive")
}
```

#### 6.2.3 Output Configuration

**Location:** `internal/config/config.go:31-34`

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `output.format` | string | `"table"` | Output format (`table`, `json`, `yaml`) |
| `output.verbose` | bool | `false` | Enable verbose progress output |

**Validation:**

```go
// internal/config/config.go:97-101
validFormats := []string{"table", "json", "yaml"}
if !contains(validFormats, c.Output.Format) {
    return fmt.Errorf("unsupported output format '%s'", c.Output.Format)
}
```

### 6.3 Configuration Precedence

**Order (highest to lowest priority):**

1. **CLI Flags** (e.g., `--db`, `--format`, `--fail-on-risk-level`)
2. **Configuration File** (`.tapa.yml` or `--config <path>`)
3. **Environment Variables** (not implemented yet)
4. **Defaults** (in code)

**Example:**

```bash
# Config file: .tapa.yml
database:
  type: postgresql
  url: postgres://localhost/staging_db

# CLI command (overrides config)
tapa analyze migrations/ --db postgres://localhost/prod_db --format json
```

Result:
- `database.url`: `postgres://localhost/prod_db` (CLI wins)
- `database.type`: `postgresql` (from config, not overridden)
- `output.format`: `json` (CLI wins)

**Implementation:**

```go
// cmd/tapa/analyze.go:70-81
if opts.dbURL != "" {
    cfg.Database.URL = opts.dbURL  // Override
}
if opts.dbType != "" {
    cfg.Database.Type = opts.dbType
}
if opts.format != "" {
    cfg.Output.Format = opts.format
}
```

### 6.4 Environment Variables

**Currently supported:**

| Variable | Effect | Used By |
|----------|--------|---------|
| `NO_COLOR` | Disable colored output | `internal/output/colors.go` |
| `DATABASE_URL` | Database connection (via shell expansion) | User's shell |

**Future work:**

```bash
# Not yet implemented
export TAPA_DB_URL="postgres://localhost/mydb"
export TAPA_DISK_THROUGHPUT=300
export TAPA_FAIL_ON_RISK=high
```

### 6.5 CLI Flags

#### 6.5.1 Analyze Command

**Location:** `cmd/tapa/analyze.go:51-58`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--db` | string | `""` | Database connection URL |
| `--db-type` | string | `""` | Database type (auto-detected from URL) |
| `--format` | string | `"table"` | Output format (table/json/yaml) |
| `--dry-run` | bool | `false` | Analyze without DB connection |
| `--fail-on-risk-level` | string | `""` | Exit with error if risk exceeds threshold |
| `--comprehensive` | bool | `false` | Enable Phase 2 features |
| `-v, --verbose` | bool | `false` | Enable verbose output |

**Examples:**

```bash
# Basic analysis
tapa analyze migrations/001.sql --db postgres://localhost/mydb

# Comprehensive analysis
tapa analyze migrations/ --comprehensive --verbose

# CI/CD usage
tapa analyze migrations/ --format json --fail-on-risk-level high
```

#### 6.5.2 Batch Command

**Location:** `cmd/tapa/batch.go:44-48`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--db` | string | `""` | Database connection URL |
| `--db-type` | string | `""` | Database type |
| `--format` | string | `"table"` | Output format |
| `--dry-run` | bool | `false` | Analyze without DB connection |

**Examples:**

```bash
# Generate batching strategy
tapa batch migrations/ --db-type postgresql

# Export to JSON
tapa batch migrations/ --format json > batches.json
```

---

## 7. Extension Points

### 7.1 Adding a New Database Type

**Steps:**

1. **Implement Parser Interface**

   ```go
   // internal/parser/newdb/parser.go
   package newdb
   
   type Parser struct{}
   
   func NewParser() *Parser {
       return &Parser{}
   }
   
   func (p *Parser) Parse(sql string) ([]*models.Operation, error) {
       // Use database-specific SQL parser library
       // Return []models.Operation
   }
   
   func (p *Parser) ParseFile(filePath string) (*models.Migration, error) {
       // Read file, call Parse()
   }
   ```

2. **Implement Analyzer Interface**

   ```go
   // internal/analyzer/newdb/analyzer.go
   package newdb
   
   type Analyzer struct {
       introspector db.Introspector
   }
   
   func NewAnalyzer(introspector db.Introspector) *Analyzer {
       return &Analyzer{introspector: introspector}
   }
   
   func (a *Analyzer) Analyze(ctx context.Context, op *models.Operation) error {
       // Step 1: Detect lock type
       // Step 2: Get table stats
       // Step 3: Estimate duration
       // Step 4: Calculate risk score
       // Step 5: Set backward compatibility
       // Step 6: Generate recommendations
       return nil
   }
   ```

3. **Implement Introspector Interface**

   ```go
   // internal/db/newdb/introspector.go
   package newdb
   
   type Introspector struct {
       connStr string
       conn    *sql.DB
   }
   
   func NewIntrospector(connStr string) *Introspector {
       return &Introspector{connStr: connStr}
   }
   
   func (i *Introspector) Connect(ctx context.Context) error {
       // Open database connection
   }
   
   func (i *Introspector) GetTableStats(ctx context.Context, tableName string) (*db.TableStats, error) {
       // Query table statistics from system catalogs
   }
   ```

4. **Register in Factory Functions**

   ```go
   // internal/parser/parser.go
   func GetParser(dbType string) (Parser, error) {
       switch dbType {
       case "newdb":
           return newdb.NewParser(), nil
       // ...
       }
   }
   
   // internal/analyzer/analyzer.go
   func GetAnalyzer(dbType string, introspector db.Introspector, ...) (Analyzer, error) {
       switch dbType {
       case "newdb":
           return newdb.NewAnalyzer(introspector, ...), nil
       // ...
       }
   }
   
   // internal/introspector/factory.go
   func GetIntrospector(dbType string, connStr string) (db.Introspector, error) {
       switch dbType {
       case "newdb":
           return newdb.NewIntrospector(connStr), nil
       // ...
       }
   }
   ```

5. **Update Configuration Validation**

   ```go
   // internal/config/config.go
   validDatabaseTypes := []string{"postgresql", "mysql", "newdb"}
   ```

6. **Add Tests**

   ```go
   // internal/parser/newdb/parser_test.go
   // internal/analyzer/newdb/analyzer_test.go
   // internal/db/newdb/introspector_test.go
   ```

**Required Parser Library:**

- For SQL parsing, use database-specific libraries:
  - PostgreSQL: `github.com/pganalyze/pg_query_go`
  - MySQL: `vitess.io/vitess/go/vt/sqlparser`
  - Oracle: `github.com/xwb1989/sqlparser` (fork)
  - SQL Server: Custom regex parser (no good Go library)

---

### 7.2 Adding a New Operation Type

**Example: ADD CONSTRAINT**

1. **Add OperationType Constant**

   ```go
   // pkg/models/operation.go
   const (
       OperationTypeAddConstraint OperationType = "ADD_CONSTRAINT"
   )
   ```

2. **Update Parser to Detect Operation**

   ```go
   // internal/parser/postgres/postgres.go
   case *pg_query.Node_AlterTableCmd:
       switch c.AlterTableCmd.Subtype {
       case pg_query.AlterTableType_AT_AddConstraint:
           op.Type = models.OperationTypeAddConstraint
       }
   ```

3. **Update Analyzer to Handle Operation**

   ```go
   // internal/analyzer/postgres/analyzer.go
   func (a *Analyzer) detectLockType(op *models.Operation) {
       switch op.Type {
       case models.OperationTypeAddConstraint:
           sqlLower := strings.ToLower(op.SQL)
           if strings.Contains(sqlLower, "not valid") {
               // ADD CONSTRAINT ... NOT VALID (PostgreSQL 9.2+)
               op.LockType = models.LockTypeShareUpdateExclusive
               op.LockDurationMS = 100
           } else {
               // Regular constraint requires table scan
               op.LockType = models.LockTypeAccessExclusive
               op.RequiresRewrite = false
               op.LockDurationMS = 5000  // Depends on table size
           }
       }
   }
   ```

4. **Add Risk Calculation Logic**

   ```go
   func (a *Analyzer) calculateRiskScore(op *models.Operation, stats *db.TableStats) {
       // Existing logic...
       
       // Constraint-specific risk
       if op.Type == models.OperationTypeAddConstraint {
           // Table scan required for validation
           if stats != nil {
               scanTime := calculateScanTime(stats)
               op.EstimatedTimeSeconds = scanTime
           }
       }
   }
   ```

5. **Add Recommendations**

   ```go
   func (a *Analyzer) generateRecommendations(op *models.Operation, stats *db.TableStats) {
       switch op.Type {
       case models.OperationTypeAddConstraint:
           op.Recommendations = append(op.Recommendations,
               "Use NOT VALID to add constraint without table scan, then VALIDATE CONSTRAINT separately")
       }
   }
   ```

6. **Add Tests**

   ```go
   // internal/analyzer/postgres/analyzer_test.go
   func TestAnalyzeAddConstraint(t *testing.T) {
       // Test cases for ADD CONSTRAINT
   }
   ```

---

### 7.3 Adding a New Output Formatter

**Example: Markdown Formatter**

1. **Implement Formatter Function**

   ```go
   // internal/output/markdown.go
   package output
   
   import (
       "fmt"
       "io"
       "github.com/iamsr/tapa/pkg/models"
   )
   
   // FormatMarkdown outputs result as GitHub-flavored Markdown
   func FormatMarkdown(w io.Writer, result *models.AnalysisResult) error {
       fmt.Fprintln(w, "# Migration Analysis Report")
       fmt.Fprintln(w, "")
       
       for _, migration := range result.Migrations {
           fmt.Fprintf(w, "## %s\n\n", migration.FilePath)
           fmt.Fprintln(w, "| Operation | Table | Risk | Time | Recommendations |")
           fmt.Fprintln(w, "|-----------|-------|------|------|-----------------|")
           
           for _, op := range migration.Operations {
               fmt.Fprintf(w, "| %s | %s | %s (%d) | %.1fs | %s |\n",
                   op.Type,
                   op.TableName,
                   op.RiskLevel(),
                   op.RiskScore,
                   op.EstimatedTimeSeconds,
                   joinRecommendations(op.Recommendations),
               )
           }
           
           fmt.Fprintln(w, "")
       }
       
       return nil
   }
   
   func joinRecommendations(recs []string) string {
       if len(recs) == 0 {
           return "-"
       }
       return strings.Join(recs, "; ")
   }
   ```

2. **Register in Format Function**

   ```go
   // internal/output/formatter.go
   func Format(w io.Writer, result *models.AnalysisResult, format string) error {
       switch format {
       case "markdown":
           return FormatMarkdown(w, result)
       // ...
       }
   }
   ```

3. **Add to Configuration Validation**

   ```go
   // internal/config/config.go
   validFormats := []string{"table", "json", "yaml", "markdown"}
   ```

4. **Add Tests**

   ```go
   // internal/output/markdown_test.go
   func TestFormatMarkdown(t *testing.T) {
       // Test markdown output
   }
   ```

---

### 7.4 Adding Phase 2 Modules

**Phase 2 modules** (dependencies, time estimator, alternatives, batcher) use a plugin-style architecture.

**Example: Custom Dependency Analyzer**

1. **Implement DependencyAnalyzer Interface**

   ```go
   // internal/analyzer/dependencies/custom.go
   package dependencies
   
   type CustomAnalyzer struct {
       introspector db.Introspector
   }
   
   func NewCustomAnalyzer(introspector db.Introspector) *CustomAnalyzer {
       return &CustomAnalyzer{introspector: introspector}
   }
   
   func (a *CustomAnalyzer) FindDependencies(ctx context.Context, op *models.Operation) ([]models.Dependency, error) {
       var deps []models.Dependency
       
       // Custom logic to find dependencies
       // Example: Check for application-specific metadata tables
       
       return deps, nil
   }
   ```

2. **Register in Factory**

   ```go
   // internal/analyzer/dependencies/analyzer.go
   func GetDependencyAnalyzer(dbType string, introspector db.Introspector) (DependencyAnalyzer, error) {
       switch dbType {
       case "custom":
           return NewCustomAnalyzer(introspector), nil
       // ...
       }
   }
   ```

3. **Use in Analyzer**

   ```go
   // internal/analyzer/postgres/analyzer.go
   analyzer.dependencyAnalyzer, _ = dependencies.GetDependencyAnalyzer("postgresql", introspector)
   
   // In AnalyzeWithEnhancements()
   if opts.IncludeDependencies && a.dependencyAnalyzer != nil {
       deps, err := a.dependencyAnalyzer.FindDependencies(ctx, op)
       if err == nil {
           op.Dependencies = deps
       }
   }
   ```

---

## 8. Testing Architecture

### 8.1 Test Organization

TAPA uses a comprehensive 4-tier testing strategy:

```
tests/
├── unit/           # Unit tests (co-located with code)
├── integration/    # Integration tests (Docker-based)
├── e2e/            # End-to-end tests (full CLI)
└── ci/             # CI/CD integration tests

internal/
└── */
    └── *_test.go   # Unit tests co-located with code
```

### 8.2 Unit Tests

**Location:** Co-located with source files (`*_test.go`)

**Examples:**

```
internal/analyzer/postgres/analyzer_test.go
internal/parser/postgres/parser_test.go
pkg/models/operation_test.go
```

**Run Unit Tests:**

```bash
# All unit tests
go test ./... -short

# Specific package
go test ./internal/analyzer/postgres -v

# With coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

**Example Unit Test:**

```go
// internal/analyzer/postgres/analyzer_test.go
func TestAnalyzeAddColumn(t *testing.T) {
    analyzer := NewAnalyzer(nil, 200, 2.0)
    
    op := &models.Operation{
        SQL:       "ALTER TABLE users ADD COLUMN email VARCHAR(255)",
        Type:      models.OperationTypeAddColumn,
        TableName: "users",
    }
    
    err := analyzer.Analyze(context.Background(), op)
    assert.NoError(t, err)
    assert.Equal(t, models.LockTypeAccessExclusive, op.LockType)
    assert.False(t, op.RequiresRewrite)
    assert.True(t, op.BackwardCompatible)
}
```

### 8.3 Integration Tests

**Location:** `tests/e2e/docker-compose.yml` (Docker-based databases)

**Setup:**

```yaml
# tests/e2e/docker-compose.yml
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: testpass
      POSTGRES_DB: testdb
    ports:
      - "5432:5432"
  
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: testpass
      MYSQL_DATABASE: testdb
    ports:
      - "3306:3306"
```

**Run Integration Tests:**

```bash
# Start databases
cd tests/e2e
docker-compose up -d

# Wait for databases to be ready
sleep 10

# Run integration tests
go test ./internal/analyzer/postgres -v -run Integration
go test ./internal/analyzer/mysql -v -run Integration

# Cleanup
docker-compose down -v
```

**Example Integration Test:**

```go
// internal/analyzer/postgres/analyzer_test.go
func TestIntegrationPostgresIntrospection(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    // Connect to test database
    connStr := "postgres://postgres:testpass@localhost:5432/testdb?sslmode=disable"
    introspector := postgres.NewIntrospector(connStr)
    defer introspector.Close()
    
    ctx := context.Background()
    err := introspector.Connect(ctx)
    require.NoError(t, err)
    
    // Create test table
    _, err = introspector.conn.ExecContext(ctx, `
        CREATE TABLE test_users (
            id SERIAL PRIMARY KEY,
            email VARCHAR(255)
        )
    `)
    require.NoError(t, err)
    
    // Test introspection
    stats, err := introspector.GetTableStats(ctx, "test_users")
    require.NoError(t, err)
    assert.Equal(t, "test_users", stats.TableName)
    assert.GreaterOrEqual(t, stats.TableSizeBytes, int64(0))
}
```

### 8.4 End-to-End Tests

**Location:** `tests/e2e/`

Tests the full CLI workflow with real migration files.

**Run E2E Tests:**

```bash
cd tests/e2e
./run_tests.sh
```

**Example E2E Test:**

```bash
# tests/e2e/test_analyze.sh
#!/bin/bash

# Setup
docker-compose up -d
trap 'docker-compose down -v' EXIT

# Wait for databases
sleep 10

# Test PostgreSQL analysis
echo "Testing PostgreSQL analysis..."
../../tapa analyze fixtures/postgres/001_add_column.sql \
    --db "postgres://postgres:testpass@localhost:5432/testdb?sslmode=disable" \
    --format json > result_pg.json

# Verify output
jq -e '.migrations[0].operations[0].risk_score' result_pg.json
jq -e '.migrations[0].operations[0].lock_type == "ACCESS_EXCLUSIVE"' result_pg.json

# Test MySQL analysis
echo "Testing MySQL analysis..."
../../tapa analyze fixtures/mysql/001_add_column.sql \
    --db "mysql://root:testpass@tcp(localhost:3306)/testdb" \
    --format json > result_mysql.json

jq -e '.migrations[0].operations[0].risk_score' result_mysql.json

echo "E2E tests passed!"
```

### 8.5 CI/CD Tests

**Location:** `tests/ci/`

Tests GitHub Actions and GitLab CI integration scripts.

**Run CI Tests:**

```bash
# GitHub Actions
bash tests/ci/test-github-action.sh

# GitLab CI
bash tests/ci/test-gitlab-ci.sh
```

**Example CI Test:**

```bash
# tests/ci/test-github-action.sh
#!/bin/bash

# Simulate GitHub Actions environment
export GITHUB_WORKSPACE=/tmp/test-workspace
export GITHUB_OUTPUT=/tmp/github-output

# Setup test files
mkdir -p $GITHUB_WORKSPACE/migrations
cat > $GITHUB_WORKSPACE/migrations/001.sql <<EOF
ALTER TABLE users ADD COLUMN email VARCHAR(255);
EOF

# Run action
.github/actions/tapa-analyzer/action.sh \
    --migration-path="migrations/" \
    --db-type="postgresql" \
    --fail-on-risk="high"

# Verify outputs
grep -q "risk_level=" $GITHUB_OUTPUT
grep -q "max_risk_score=" $GITHUB_OUTPUT

echo "CI test passed!"
```

### 8.6 Test Coverage Goals

| Component | Target Coverage | Current Status |
|-----------|-----------------|----------------|
| Parser | 90%+ | ✅ Achieved |
| Analyzer | 90%+ | ✅ Achieved |
| Introspector | 80%+ | ✅ Achieved |
| Output Formatters | 85%+ | ✅ Achieved |
| CLI Commands | 70%+ | ✅ Achieved |
| Overall | 85%+ | ✅ 87% |

**View Coverage:**

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### 8.7 Testing Best Practices

1. **Table-Driven Tests**

   ```go
   func TestRiskScoreCalculation(t *testing.T) {
       tests := []struct {
           name          string
           lockType      models.LockType
           tableSizeGB   float64
           durationMins  float64
           expectedScore int
       }{
           {"Small table, fast", models.LockTypeNone, 1, 1, 3},
           {"Large table, slow", models.LockTypeAccessExclusive, 100, 60, 100},
       }
       
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               // Test logic
           })
       }
   }
   ```

2. **Mock Introspector for Unit Tests**

   ```go
   type mockIntrospector struct{}
   
   func (m *mockIntrospector) GetTableStats(ctx context.Context, tableName string) (*db.TableStats, error) {
       return &db.TableStats{
           TableName:      tableName,
           RowCount:       1000000,
           TableSizeBytes: 10 * 1024 * 1024 * 1024,
       }, nil
   }
   ```

3. **Golden Files for Output Tests**

   ```go
   func TestFormatTable(t *testing.T) {
       result := createTestResult()
       
       var buf bytes.Buffer
       err := FormatTable(&buf, result)
       require.NoError(t, err)
       
       golden.Assert(t, buf.String(), "testdata/format_table.golden")
   }
   ```

4. **Cleanup in Tests**

   ```go
   func TestDatabaseOperations(t *testing.T) {
       // Setup
       db := setupTestDB(t)
       defer db.Close()
       
       // Create test table
       _, err := db.Exec("CREATE TABLE test_table ...")
       require.NoError(t, err)
       defer db.Exec("DROP TABLE test_table")
       
       // Test logic
   }
   ```

---

## 9. Performance Considerations

### 9.1 Parsing Performance

**Bottleneck:** SQL parsing with pg_query/vitess

**Optimization:**
- Parsing is single-threaded per file
- Multiple files can be parsed in parallel (future work)

**Current Performance:**

| File Size | Operations | Parse Time |
|-----------|------------|------------|
| 1 KB | 1 op | ~5ms |
| 10 KB | 10 ops | ~20ms |
| 100 KB | 100 ops | ~150ms |

**Optimization Opportunity:**

```go
// Future: Parallel file parsing
func parseFilesParallel(files []string, parser Parser) ([]*models.Migration, error) {
    results := make(chan *models.Migration, len(files))
    errors := make(chan error, len(files))
    
    for _, file := range files {
        go func(f string) {
            migration, err := parser.ParseFile(f)
            if err != nil {
                errors <- err
                return
            }
            results <- migration
        }(file)
    }
    
    // Collect results
}
```

### 9.2 Database Introspection

**Bottleneck:** Network latency for database queries

**Current Approach:**
- One query per table (GetTableStats)
- Queries are sequential

**Optimization:**

```go
// Cache table stats for repeated analysis
type cachedIntrospector struct {
    introspector db.Introspector
    cache        map[string]*db.TableStats
    mu           sync.RWMutex
}

func (c *cachedIntrospector) GetTableStats(ctx context.Context, tableName string) (*db.TableStats, error) {
    c.mu.RLock()
    if stats, ok := c.cache[tableName]; ok {
        c.mu.RUnlock()
        return stats, nil
    }
    c.mu.RUnlock()
    
    // Cache miss, query database
    stats, err := c.introspector.GetTableStats(ctx, tableName)
    if err == nil {
        c.mu.Lock()
        c.cache[tableName] = stats
        c.mu.Unlock()
    }
    return stats, err
}
```

**Batch Queries (Future):**

```sql
-- Instead of N queries for N tables, use one query
SELECT 
    tablename,
    pg_total_relation_size(tablename::regclass) as size,
    n_live_tup as row_count
FROM pg_stat_user_tables
WHERE tablename IN ('table1', 'table2', ...);
```

### 9.3 Memory Usage

**Current Memory Profile:**

| Dataset | Memory Usage |
|---------|--------------|
| 10 migrations, 50 ops | ~5 MB |
| 100 migrations, 500 ops | ~50 MB |
| 1000 migrations, 5000 ops | ~500 MB |

**Memory Growth Factors:**
- SQL strings (stored in Operation.SQL)
- Recommendations (string arrays)
- Dependencies (arrays of Dependency objects)

**Optimization Opportunity:**

```go
// Stream processing for large codebases
func analyzeStream(files []string, outputWriter io.Writer) error {
    for _, file := range files {
        // Parse file
        migration, err := parser.ParseFile(file)
        
        // Analyze operations
        for _, op := range migration.Operations {
            analyzer.Analyze(ctx, op)
        }
        
        // Output immediately (don't accumulate in memory)
        output.FormatSingleMigration(outputWriter, migration)
        
        // Release memory
        migration = nil
    }
}
```

### 9.4 Dry-Run Mode Performance

**Dry-run mode** (no database connection) is significantly faster:

| Mode | 100 Operations | Speedup |
|------|----------------|---------|
| With DB | ~5 seconds | 1x |
| Dry-run | ~0.5 seconds | 10x |

**Use Cases for Dry-Run:**
- CI/CD pre-merge checks (no DB access)
- Quick local validation
- Large codebases (1000+ migrations)

**Trade-off:**
- Less accurate time estimates (uses conservative fallback values)

### 9.5 Output Performance

**JSON/YAML output** is faster than table formatting:

| Format | 100 Operations | Time |
|--------|----------------|------|
| JSON | 100 ops | ~10ms |
| YAML | 100 ops | ~15ms |
| Table | 100 ops | ~50ms (color codes, borders) |

**Optimization:**
- Use JSON format in CI/CD pipelines
- Disable colors with `NO_COLOR=1` for faster table output

---

## 10. Security

### 10.1 Credential Handling

**Database Credentials:**

TAPA accepts database URLs with embedded credentials:

```bash
tapa analyze migrations/ --db "postgres://user:password@host/db"
```

**Security Measures:**

1. **No Credential Storage**
   - Credentials are never written to disk
   - Not cached in memory after connection

2. **Command-Line Exposure Risk**
   - Credentials visible in process list (`ps aux`)
   - Mitigation: Use environment variables

   ```bash
   export DB_URL="postgres://user:password@host/db"
   tapa analyze migrations/ --db "$DB_URL"
   ```

3. **Configuration File Security**
   - `.tapa.yml` should not be committed to version control
   - Add to `.gitignore`:

   ```gitignore
   .tapa.yml
   .env
   ```

4. **Connection String Sanitization (Future)**

   ```go
   // Redact password from logs
   func sanitizeURL(url string) string {
       u, _ := url.Parse(url)
       if u.User != nil {
           u.User = url.UserPassword(u.User.Username(), "***")
       }
       return u.String()
   }
   ```

### 10.2 SQL Injection Prevention

**Risk:** TAPA analyzes user-provided SQL files

**Mitigation:**

1. **No SQL Execution**
   - TAPA only **parses** SQL, never executes it
   - Analysis is read-only

2. **Parameterized Queries for Introspection**

   ```go
   // internal/db/postgres/introspector.go (pseudo-code)
   func (i *Introspector) GetTableStats(ctx context.Context, tableName string) (*db.TableStats, error) {
       // SAFE: Uses parameterized query
       query := "SELECT * FROM pg_class WHERE relname = $1"
       row := i.conn.QueryRowContext(ctx, query, tableName)
       
       // UNSAFE (not used in TAPA):
       // query := fmt.Sprintf("SELECT * FROM pg_class WHERE relname = '%s'", tableName)
   }
   ```

3. **Input Validation**

   ```go
   // Validate table name format
   func isValidTableName(name string) bool {
       // Allow: alphanumeric, underscore, dot (schema-qualified)
       pattern := regexp.MustCompile(`^[a-zA-Z0-9_]+(\.[a-zA-Z0-9_]+)?$`)
       return pattern.MatchString(name)
   }
   ```

### 10.3 File System Access

**Risk:** TAPA reads migration files from disk

**Security Measures:**

1. **Path Traversal Prevention**

   ```go
   // cmd/tapa/analyze.go:260-284
   func findMigrationFiles(path string) ([]string, error) {
       // Use filepath.Walk (safe, doesn't follow symlinks by default)
       err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
           // Only read .sql files
           if !info.IsDir() && strings.HasSuffix(strings.ToLower(p), ".sql") {
               files = append(files, p)
           }
           return nil
       })
   }
   ```

2. **Symlink Handling**
   - `filepath.Walk` does not follow symlinks by default
   - Future: Add `--follow-symlinks` flag if needed

3. **File Size Limits (Future)**

   ```go
   const maxFileSizeBytes = 10 * 1024 * 1024 // 10 MB
   
   func (p *Parser) ParseFile(filePath string) (*models.Migration, error) {
       info, err := os.Stat(filePath)
       if err != nil {
           return nil, err
       }
       
       if info.Size() > maxFileSizeBytes {
           return nil, fmt.Errorf("file too large: %d bytes", info.Size())
       }
       
       // Proceed with parsing
   }
   ```

### 10.4 Database Connection Security

**Encryption:**

- PostgreSQL: Use `?sslmode=require` in connection string
- MySQL: Use TLS parameters in DSN

**Example:**

```bash
# PostgreSQL with SSL
--db "postgres://user:pass@host/db?sslmode=require"

# MySQL with TLS
--db "mysql://user:pass@tcp(host:3306)/db?tls=true"
```

**Read-Only Access:**

```sql
-- PostgreSQL: Grant read-only access
CREATE USER tapa_user WITH PASSWORD 'secure_password';
GRANT CONNECT ON DATABASE mydb TO tapa_user;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO tapa_user;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO tapa_user;

-- MySQL: Grant read-only access
CREATE USER 'tapa_user'@'%' IDENTIFIED BY 'secure_password';
GRANT SELECT ON mydb.* TO 'tapa_user'@'%';
FLUSH PRIVILEGES;
```

**Connection Timeouts:**

```go
// internal/db/postgres/introspector.go (future)
connStr := fmt.Sprintf("%s?connect_timeout=10", originalConnStr)
```

### 10.5 CI/CD Security

**GitHub Actions:**

```yaml
# .github/workflows/migration-analysis.yml
- uses: ./.github/actions/tapa-analyzer
  with:
    github-token: ${{ secrets.GITHUB_TOKEN }}
    # Don't expose database credentials
    db-type: postgresql
    # Use dry-run mode in CI
    dry-run: true
```

**GitLab CI:**

```yaml
# .gitlab-ci.yml
migration-analysis:
  script:
    - tapa analyze migrations/ --dry-run --format json
  # Don't expose credentials in CI logs
```

**Secret Management:**

- Store credentials in GitHub Secrets / GitLab Variables
- Use read-only database accounts
- Prefer dry-run mode when possible

---

## 11. Design Decisions

### 11.1 Why Static Analysis?

**Decision:** Analyze SQL files without executing them

**Rationale:**
- **Safety:** No risk of accidental execution in production
- **Speed:** Faster than test migrations (no database setup)
- **Portability:** Works offline, in CI/CD without database access
- **Early Feedback:** Catch issues during code review, not deployment

**Trade-offs:**
- Less accurate than actual execution
- Cannot detect runtime-specific issues (e.g., constraint violations)

**Alternative Considered:** Schema diff tool (compare before/after schemas)
- **Rejected:** Requires database access, slower, harder to integrate into CI/CD

---

### 11.2 Why Parser Libraries Instead of Regex?

**Decision:** Use pg_query (PostgreSQL) and Vitess (MySQL) for parsing

**Rationale:**
- **Accuracy:** Handles complex SQL syntax (nested subqueries, CTEs, etc.)
- **Maintainability:** No need to update regex for new SQL features
- **Type Safety:** Structured AST instead of string manipulation

**Example of Regex Limitations:**

```sql
-- Regex would struggle with this
ALTER TABLE users 
    ADD COLUMN email VARCHAR(255) DEFAULT 'test@example.com',
    ADD COLUMN phone VARCHAR(20) DEFAULT NULL,
    DROP COLUMN fax;
```

Parser AST provides:
- Individual operations (3 separate ALTER TABLE commands)
- Column types, defaults, constraints

**Trade-offs:**
- Larger binary size (pg_query is a C library via CGO)
- Platform-specific builds

---

### 11.3 Why Risk Score Formula?

**Decision:** Risk = baseLockScore (40) + tableSizeScore (30) + durationScore (30)

**Rationale:**

1. **Lock Type (40 points):** Most critical factor
   - ACCESS EXCLUSIVE = 40 → Blocks everything
   - SHARE = 20 → Blocks writes only
   - NONE = 0 → Non-blocking

2. **Table Size (30 points):** Scales with impact
   - Large tables = more users affected
   - Capped at 30 to avoid overshadowing lock type

3. **Duration (30 points):** Time at risk
   - Longer operations = higher chance of incidents
   - Capped at 30 for balance

**Alternative Considered:** Multiplicative formula (`lock * size * duration`)
- **Rejected:** Too aggressive, small tables with short durations would have near-zero risk

**Calibration:**

| Scenario | Expected Risk Level | Actual Score |
|----------|---------------------|--------------|
| Add nullable column (small table) | LOW | 40 → MEDIUM |
| Add column with DEFAULT (large table) | CRITICAL | 85 → CRITICAL |
| Create index concurrently (large table) | MEDIUM | 40 → MEDIUM |
| Drop table (any size) | MEDIUM-HIGH | 40-50 → MEDIUM |

**Future Tuning:**
- Adjust weights based on real-world incidents
- Add database version factor (PG 11+ has instant operations)

---

### 11.4 Why Conservative Estimates?

**Decision:** Use conservative fallback values when database is unavailable

**Rationale:**
- **Safety First:** Overestimate risk rather than underestimate
- **Dry-Run Mode:** Still provides value without database access
- **Unknown Unknowns:** Production tables may be larger than dev/staging

**Fallback Values:**

```go
// internal/analyzer/postgres/analyzer.go:59-72
stats = &db.TableStats{
    RowCount:       1000000,                 // 1M rows
    TableSizeBytes: 10 * 1024 * 1024 * 1024, // 10 GB
}
```

**Example Impact:**

```sql
ALTER TABLE users ADD COLUMN status VARCHAR(50) DEFAULT 'active';
```

- **With introspection (1 GB table):** Risk = 70 (HIGH)
- **Dry-run mode (assumes 10 GB):** Risk = 85 (CRITICAL)

**Trade-off:**
- May cause false positives (flagging low-risk operations as high-risk)
- Better than false negatives (missing high-risk operations)

---

### 11.5 Why Separate Commands (analyze vs batch)?

**Decision:** Two separate commands instead of one unified command

**Rationale:**

1. **Different Use Cases:**
   - `analyze` → Detailed per-operation analysis
   - `batch` → High-level deployment strategy

2. **Output Clarity:**
   - `analyze` → Focus on lock types, risk scores, recommendations
   - `batch` → Focus on grouping, prerequisites, deployment order

3. **CLI Simplicity:**
   - Avoid complex `--mode` flags
   - Clear intent from command name

**Alternative Considered:** Single command with `--batching` flag
- **Rejected:** Confusion about output format, harder to document

**Example:**

```bash
# Detailed analysis for code review
tapa analyze migrations/001.sql

# Deployment planning for release
tapa batch migrations/
```

---

### 11.6 Why Phase 1 vs Phase 2 Features?

**Decision:** Split features into basic (Phase 1) and comprehensive (Phase 2)

**Rationale:**

1. **Performance:** Phase 2 features add overhead
   - Dependency analysis requires additional queries
   - Alternative generation involves complex logic

2. **Simplicity:** Default behavior should be fast and simple
   - Most users only need risk scores and time estimates
   - Power users can opt-in with `--comprehensive`

3. **Incremental Adoption:**
   - Phase 1 provides immediate value
   - Phase 2 can be added gradually

**Phase 1 (Default):**
- Lock detection
- Risk scoring
- Time estimation
- Basic recommendations

**Phase 2 (`--comprehensive`):**
- Dependency analysis
- Time breakdown
- Alternative strategies

**Trade-off:**
- More complex CLI (two modes)
- Documentation burden (explain when to use each mode)

---

### 11.7 Why Not Execute Migrations?

**Decision:** TAPA is analysis-only, does not execute migrations

**Rationale:**

1. **Safety:** Analysis tool should not modify production
2. **Separation of Concerns:** Use dedicated migration tools (Flyway, Liquibase, migrate)
3. **Integration:** TAPA complements existing workflows, doesn't replace them

**Workflow:**

```
1. Write migration file
2. Run TAPA analyze → Get risk assessment
3. Review recommendations
4. Execute migration with existing tool (psql, mysql, Flyway, etc.)
```

**Alternative Considered:** Add `tapa execute` command
- **Rejected:** Out of scope, many good migration tools already exist

---

### 11.8 Why Support Dry-Run Mode?

**Decision:** Allow analysis without database connection

**Rationale:**

1. **CI/CD Integration:** Many CI environments don't have database access
2. **Pre-Merge Checks:** Analyze PRs before merging (GitHub Actions)
3. **Offline Development:** Work without database running
4. **Security:** Avoid exposing production credentials to CI

**Trade-off:**
- Less accurate risk scores (uses conservative estimates)
- Cannot detect table-specific issues

**Use Cases:**

| Scenario | Dry-Run? | Database? |
|----------|----------|-----------|
| Local development | No | Staging DB |
| PR review (GitHub Actions) | Yes | None |
| Pre-deployment validation | No | Production replica |
| Quick syntax check | Yes | None |

---

### 11.9 Why JSON/YAML Output?

**Decision:** Support machine-readable output formats

**Rationale:**

1. **Automation:** Parse results in scripts, CI/CD pipelines
2. **Integration:** Feed results into other tools (dashboards, alerting)
3. **Archiving:** Store analysis results for historical comparison

**Example Integration:**

```bash
# Generate report
tapa analyze migrations/ --format json > report.json

# Parse in script
HIGH_RISK=$(jq '[.migrations[].operations[] | select(.risk_score >= 51)] | length' report.json)

if [ "$HIGH_RISK" -gt 0 ]; then
    echo "Found $HIGH_RISK high-risk operations"
    exit 1
fi
```

**Alternative Considered:** Custom binary format
- **Rejected:** JSON/YAML are universal, human-readable when needed

---

### 11.10 Why Modular Architecture?

**Decision:** Separate parser, analyzer, introspector, output modules

**Rationale:**

1. **Extensibility:** Easy to add new databases (just implement interfaces)
2. **Testing:** Mock individual components (e.g., mock introspector for unit tests)
3. **Maintainability:** Clear boundaries, single responsibility principle

**Architecture Pattern:** Factory + Strategy

```
GetParser(dbType) → Parser interface → Database-specific implementation
GetAnalyzer(dbType) → Analyzer interface → Database-specific implementation
```

**Benefits:**
- Add PostgreSQL 17 support → Update postgres.Parser only
- Add Oracle support → Implement new oracle.Parser + oracle.Analyzer
- Change risk formula → Update analyzer only, parser/introspector unchanged

**Trade-off:**
- More files, more abstraction
- Overhead for small features (simple changes touch multiple files)

---

## Conclusion

This architecture document provides a comprehensive guide to TAPA's design, implementation, and extension points. Key takeaways:

1. **Modular Design:** Parser, Analyzer, Introspector, Batcher modules with clear interfaces
2. **Risk-First Approach:** Risk scoring formula balances lock type, table size, and duration
3. **Database Agnostic:** Extensible architecture supports multiple databases (PostgreSQL, MySQL)
4. **Safety by Default:** Conservative estimates, read-only database access, no SQL execution
5. **CI/CD Ready:** JSON output, dry-run mode, fail-on-risk-level for automated checks

**For Engineers:**
- See Section 7 (Extension Points) to add new databases or operations
- See Section 8 (Testing) for testing guidelines
- See Section 4 (Algorithms) for risk scoring and time estimation details

**For AI Agents:**
- Use this document as context for understanding TAPA's codebase
- Reference specific sections (e.g., "Section 4.1 Risk Scoring") for targeted questions
- Consult Section 11 (Design Decisions) to understand architectural choices

**Next Steps:**
- Explore the codebase starting from `cmd/tapa/main.go:8`
- Run tests: `go test ./... -v`
- Try TAPA: `tapa analyze examples/postgres/001_add_column.sql`

For questions or contributions, see `CONTRIBUTING.md` and open an issue on GitHub.
