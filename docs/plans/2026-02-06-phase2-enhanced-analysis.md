# Phase 2: Enhanced Analysis & Production Safety

**Date**: 2026-02-06  
**Status**: Design Complete - Ready for Implementation  
**Duration**: 5 weeks (21 tasks)

## Overview

Transform DMA from a basic analyzer into an intelligent migration assistant that not only identifies risks but also suggests safe alternatives and execution strategies.

**Core Philosophy**: "Make the safe path the easy path" - For every high-risk operation detected, provide concrete, actionable alternatives.

## Architecture

```
┌──────────────────────────────────────────────────────┐
│           CLI Analyze Command (existing)              │
└────────────────────┬─────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────────┐
│         Core Analyzer (Phase 1 - existing)           │
│   • Lock Detection  • Risk Scoring  • Recommendations│
└────────────────────┬─────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────────┐
│              Phase 2 Enhancement Modules              │
├──────────────────────────────────────────────────────┤
│  DependencyAnalyzer    AdvancedTimeEstimator         │
│  MigrationBatcher      SafeAlternativeGenerator      │
└──────────────────────────────────────────────────────┘
```

## Module 1: Dependency Analyzer

**Purpose**: Discover what else will break when a column, table, or index is modified or dropped.

### Key Features
- Detects indexes that depend on columns
- Finds foreign key relationships
- Discovers views that reference tables/columns
- Identifies triggers and functions affected

### Implementation
- Location: `internal/analyzer/dependencies/`
- PostgreSQL queries for pg_index, pg_constraint, pg_views
- Impact levels: BREAKS, DEGRADES, SAFE

## Module 2: Advanced Time Estimator

**Purpose**: Provide accurate time predictions including index builds, constraint validation, and partition handling.

### Key Features
- Index build time calculation (btree, gin, gist)
- Constraint validation estimates
- Partition-aware calculations
- Detailed time breakdown

### Calculation Factors
- Index complexity: btree=1.0, gin=2.5, gist=2.0
- Constraint checks: NOT NULL=1x scan, CHECK=2x scan
- Partition multipliers for partitioned tables

## Module 3: Migration Batcher

**Purpose**: Split high-risk operations into multiple safer, smaller migrations for incremental deployment.

### Batching Strategy
- Group independent operations
- Isolate CRITICAL risk operations
- Respect dependencies (CREATE before ALTER)
- Suggest parallel execution where safe

### Output
- Multiple batches with risk levels
- Execution prerequisites
- Timing recommendations

## Module 4: Safe Alternative Generator

**Purpose**: Generate safer alternative SQL that achieves the same goal with lower risk.

### Patterns
1. **ADD COLUMN with DEFAULT** → Multi-step (add, backfill, set default)
2. **ALTER COLUMN TYPE** → Multi-column approach
3. **CREATE INDEX** → Add CONCURRENTLY keyword
4. **DROP COLUMN** → 3-phase deployment

### Output
- Step-by-step migration plans
- Risk reduction calculations
- Tradeoff analysis

## Data Model Extensions

### Dependencies
```go
type Dependency struct {
    Type         DependencyType  // INDEX, VIEW, FOREIGN_KEY, etc.
    Name         string
    Definition   string
    ImpactLevel  string          // "BREAKS", "DEGRADES", "SAFE"
    Description  string
}
```

### Time Breakdown
```go
type TimeBreakdown struct {
    TableRewriteSeconds    float64
    IndexBuildSeconds      float64
    ConstraintCheckSeconds float64
    MetadataUpdateSeconds  float64
    TotalSeconds           float64
}
```

### Migration Batches
```go
type MigrationBatch struct {
    BatchNumber      int
    Operations       []*Operation
    MaxRiskScore     int
    RiskLevel        RiskLevel
    TotalTimeSeconds float64
    CanRunInParallel bool
    Prerequisites    []int
    Rationale        string
}
```

### Alternative Strategies
```go
type AlternativeStrategy struct {
    StrategyName    string
    Description     string
    Steps           []AlternativeStep
    RiskReduction   int
    Tradeoffs       []string
    EstimatedTime   float64
}

type AlternativeStep struct {
    StepNumber      int
    Phase           string  // "PRE_DEPLOY", "DURING_DEPLOY", "POST_DEPLOY"
    SQL             string
    Description     string
    RequiresAppChange bool
    RiskScore       int
    EstimatedTime   float64
    CanRunOffline   bool
}
```

## CLI Enhancements

### New Flags
```bash
tapa analyze migration.sql --show-dependencies
tapa analyze migration.sql --show-time-breakdown
tapa analyze migration.sql --suggest-batches
tapa analyze migration.sql --show-alternatives
tapa analyze migration.sql --generate-alternatives --output-dir ./safe-migrations/
tapa analyze migration.sql --comprehensive  # All features
```

### Configuration
```yaml
analysis:
  enable_dependency_analysis: true
  enable_advanced_timing: true
  enable_batching: true
  enable_alternatives: true
  
  batching:
    max_batch_size: 10
    isolate_critical: true
    prefer_parallel: true
  
  alternatives:
    min_risk_threshold: 51
    include_backfill_scripts: true
    batch_size_recommendation: 10000
```

## Implementation Order

### Week 1: Dependency Analyzer
- Task 8: Dependency detection queries (3 days)
- Task 9: Integration and output (2 days)

### Week 2: Advanced Time Estimator
- Task 10: Index build calculator (2 days)
- Task 11: Constraint & partition handling (2 days)
- Task 12: Time breakdown integration (1 day)

### Week 3: Migration Batcher
- Task 13: Dependency graph builder (2 days)
- Task 14: Batch grouping algorithms (2 days)
- Task 15: Batch output formatter (1 day)

### Week 4: Safe Alternative Generator
- Task 16: Pattern detection framework (2 days)
- Task 17: Alternative generators (2 days)
- Task 18: Alternative output (1 day)

### Week 5: Integration & Polish
- Task 19: End-to-end testing (2 days)
- Task 20: Performance optimization (1 day)
- Task 21: Documentation (2 days)

## Success Metrics

- **Dependency Detection**: 95%+ accuracy
- **Time Estimation**: Within 20% of actual
- **Batching**: 30%+ risk reduction
- **Alternatives**: Valid for 80%+ high-risk ops
- **Performance**: < 5s for 50 operations

## Testing Strategy

- **Unit Tests**: Mock introspector, test calculations
- **Integration Tests**: Real PostgreSQL schemas
- **End-to-End**: Sample migrations from real projects
- **Performance**: Benchmarks for large migrations

## Rollout

1. **Week 1-2**: Experimental flags, early adopters
2. **Week 3-4**: Default enabled after validation
3. **Week 5**: Production ready with documentation
