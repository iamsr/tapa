# TAPA Recommendation System

## How It Works

TAPA's recommendation system analyzes each database operation and provides **context-specific, actionable recommendations** based on:

1. **Operation Type** (ADD_COLUMN, ALTER_COLUMN, CREATE_INDEX, etc.)
2. **Operation Properties** (RequiresRewrite, LockType, RiskScore)
3. **SQL Content** (NOT NULL, DEFAULT, constraints)
4. **Database Type** (PostgreSQL vs MySQL differences)
5. **Table Statistics** (when connected to database)

## Recommendation Logic

### ADD_COLUMN Operations

#### Scenario 1: Requires Table Rewrite
```sql
ALTER TABLE users ADD COLUMN status VARCHAR(50) DEFAULT 'active';
```

**Recommendation:**
> "Add column without DEFAULT first, then set DEFAULT separately to avoid table rewrite"

**Why:** In PostgreSQL, adding a column with a DEFAULT value requires rewriting the entire table.

**Better approach:**
```sql
-- Step 1: Add column without default (fast)
ALTER TABLE users ADD COLUMN status VARCHAR(50);

-- Step 2: Set default for future rows only (fast)
ALTER TABLE users ALTER COLUMN status SET DEFAULT 'active';

-- Step 3: Backfill existing rows (can be done in batches)
UPDATE users SET status = 'active' WHERE status IS NULL;

-- Step 4: Add NOT NULL if needed (after backfill)
ALTER TABLE users ALTER COLUMN status SET NOT NULL;
```

#### Scenario 2: NOT NULL without DEFAULT
```sql
ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL;
```

**Recommendation:**
> "Consider adding column as nullable first, then add NOT NULL constraint after backfilling"

**Why:** Adding NOT NULL immediately will fail if table has existing rows.

#### Scenario 3: Simple nullable column
```sql
ALTER TABLE users ADD COLUMN email VARCHAR(255);
```

**Recommendation:** None (this is already safe!)

---

### CREATE_INDEX Operations

#### Scenario 1: Regular index (PostgreSQL)
```sql
CREATE INDEX idx_users_email ON users(email);
```

**Recommendation:**
> "Consider using CREATE INDEX CONCURRENTLY to avoid blocking writes"

**Why:** Regular CREATE INDEX locks the table from writes.

**Better approach:**
```sql
CREATE INDEX CONCURRENTLY idx_users_email ON users(email);
```

#### Scenario 2: CONCURRENTLY already used
```sql
CREATE INDEX CONCURRENTLY idx_users_email ON users(email);
```

**Recommendation:** None (already using best practice!)

#### Scenario 3: MySQL with ALGORITHM
```sql
CREATE INDEX idx_users_email ON users(email) ALGORITHM=INPLACE;
```

**Recommendation:** MySQL-specific advice about online DDL

---

### ALTER_COLUMN / MODIFY_COLUMN Operations

#### Scenario 1: Type change
```sql
ALTER TABLE products ALTER COLUMN price TYPE DECIMAL(12,2);
```

**Recommendation:**
> "Type changes require full table rewrite - consider creating new column, migrating data, then dropping old column"

**Why:** Changing column types requires rewriting all rows.

**Better approach:**
```sql
-- Step 1: Add new column
ALTER TABLE products ADD COLUMN price_new DECIMAL(12,2);

-- Step 2: Migrate data in batches
UPDATE products SET price_new = price::DECIMAL(12,2) 
WHERE id BETWEEN 1 AND 10000;
-- Repeat for all rows in batches

-- Step 3: Switch columns (in transaction)
BEGIN;
ALTER TABLE products RENAME COLUMN price TO price_old;
ALTER TABLE products RENAME COLUMN price_new TO price;
COMMIT;

-- Step 4: Drop old column
ALTER TABLE products DROP COLUMN price_old;
```

#### Scenario 2: Adding NOT NULL
```sql
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
```

**Recommendation:**
> "Ensure all existing rows have non-null values before adding NOT NULL constraint"

---

### DROP_COLUMN Operations

#### Scenario 1: Dropping column
```sql
ALTER TABLE users DROP COLUMN obsolete_field;
```

**Recommendation:**
> "Dropping columns is irreversible and may break running code - ensure no code depends on this column"

**Additional recommendation (high risk):**
> "CRITICAL RISK: Consider performing this operation during maintenance window"

---

### Risk-Based Recommendations

TAPA also adds risk-level specific recommendations:

#### Risk Score 0-25 (LOW)
No additional warnings.

#### Risk Score 26-50 (MEDIUM)
> "MEDIUM RISK: Standard precautions apply"

#### Risk Score 51-75 (HIGH)
> "HIGH RISK: Test thoroughly in staging and perform during low-traffic period"

#### Risk Score 76-100 (CRITICAL)
> "CRITICAL RISK: Consider performing this operation during maintenance window"

---

### MySQL-Specific Recommendations

For high-risk MySQL operations, TAPA recommends **pt-online-schema-change**:

```sql
ALTER TABLE users ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'active';
```

**Additional Recommendation:**
> "Consider using pt-online-schema-change for large tables to avoid locking"
>
> pt-osc command:
> ```bash
> pt-online-schema-change \
>   --alter "ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'active'" \
>   --host=localhost --user=root \
>   D=mydb,t=users \
>   --execute
> ```

---

## Why Similar Operations Get Similar Recommendations

**This is correct behavior!**

If you have multiple similar operations:
```sql
ALTER TABLE users ADD COLUMN email VARCHAR(255);      -- Operation 1
ALTER TABLE users ADD COLUMN phone VARCHAR(20);       -- Operation 2
ALTER TABLE orders ADD COLUMN notes TEXT;             -- Operation 3
ALTER TABLE products ADD COLUMN tags TEXT;            -- Operation 4
```

All four will get similar recommendations because:
1. They're all ADD_COLUMN operations
2. They all have similar properties (rewrite behavior)
3. The same advice applies to all of them

**This is good!** It means:
- Recommendations are consistent
- You can apply the same safe pattern to all similar operations
- Reduces cognitive load (don't need different strategies for each)

---

## Customizing Recommendations

### For Different Operations
To see different recommendations, use different operation types:

```sql
-- ADD_COLUMN: "Add without DEFAULT first..."
ALTER TABLE users ADD COLUMN email VARCHAR(255) DEFAULT '';

-- CREATE_INDEX: "Use CONCURRENTLY..."
CREATE INDEX idx_users_email ON users(email);

-- ALTER_COLUMN: "Type changes require rewrite..."
ALTER TABLE users ALTER COLUMN email TYPE TEXT;

-- DROP_COLUMN: "Irreversible operation..."
ALTER TABLE users DROP COLUMN obsolete_field;

-- ADD_CONSTRAINT: "Check constraint recommendations..."
ALTER TABLE users ADD CONSTRAINT email_valid CHECK (email LIKE '%@%');
```

### Comprehensive Analysis
For even more detailed recommendations, use `--comprehensive`:

```bash
tapa analyze migration.sql --db $DATABASE_URL --comprehensive
```

This adds:
- Dependency analysis
- Alternative approaches
- Time breakdown
- More detailed safety checks

---

## Recommendation Sources

Recommendations come from:

1. **PostgreSQL Analyzer** (`internal/analyzer/postgres/analyzer.go`)
   - generateRecommendations() function
   - Operation-specific logic

2. **MySQL Analyzer** (`internal/analyzer/mysql/analyzer.go`)
   - generateRecommendations() function
   - pt-osc integration

3. **Common Analyzer** (`internal/analyzer/analyzer.go`)
   - Risk-based warnings
   - General safety recommendations

---

## Example: Diverse Recommendations

To see variety in recommendations, use a migration with diverse operations:

```sql
-- Different operation types
ALTER TABLE users ADD COLUMN email VARCHAR(255) DEFAULT '';
CREATE INDEX idx_users_email ON users(email);
ALTER TABLE users ALTER COLUMN username TYPE VARCHAR(100);
DROP TABLE obsolete_table;
ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id);
CREATE TABLE audit_log (id SERIAL, event TEXT, created_at TIMESTAMP);
```

Run analysis:
```bash
tapa analyze diverse_migration.sql --db $DATABASE_URL
```

You'll see different recommendations for each operation type!

---

## Summary

✅ **Recommendation system is working correctly**
✅ **Similar operations → similar recommendations = consistency**
✅ **Different operations → different recommendations = specificity**
✅ **Risk-based warnings added for all high-risk operations**

If you want to see more variety, use a migration file with diverse operation types!
