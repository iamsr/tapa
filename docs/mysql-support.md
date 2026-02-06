# MySQL Support

DMA provides comprehensive support for MySQL 5.7+ migrations with native parser and analysis capabilities.

## Features

### Native MySQL Parser

- Built on Vitess SQL parser
- Full MySQL syntax support (5.7, 8.0+)
- `ALGORITHM` and `LOCK` clause detection
- Online DDL operation recognition

### Analysis Capabilities

#### Algorithm Detection
- **COPY**: Full table rebuild required
- **INPLACE**: Online operation without table copy
- **INSTANT**: Metadata-only change (MySQL 8.0+)

#### Lock Mode Analysis
- **NONE**: Fully online, no locks
- **SHARED**: Read-only during operation
- **EXCLUSIVE**: Table locked for writes

#### Operation Classification
- Table rewrites vs. online operations
- Index builds with estimated duration
- Column modifications with compatibility checks

### Schema Introspection

DMA queries your live MySQL database for:
- Table row counts and data size
- Index definitions and cardinality
- Foreign key relationships
- Character set and collation info

### pt-online-schema-change Integration

For operations requiring zero-downtime:

```bash
dma analyze migration.sql --db mysql://localhost/mydb --use-pt-osc
```

DMA automatically:
- Detects operations requiring pt-osc
- Generates safe pt-osc commands
- Validates pt-osc availability
- Estimates migration time

## Usage Examples

### Basic Analysis

```bash
# Analyze single file
dma analyze migrations/001_add_index.sql --db mysql://root@localhost/mydb

# Analyze directory
dma analyze migrations/ --db mysql://user:pass@host:3306/dbname
```

### With pt-online-schema-change

```bash
# Use pt-osc for safe migrations
dma analyze migrations/ --db mysql://localhost/mydb --use-pt-osc

# Dry run to see commands
dma analyze migrations/ --db mysql://localhost/mydb --use-pt-osc --dry-run
```

### JSON Output for CI/CD

```bash
dma analyze migrations/ --db $MYSQL_URL --format json > report.json
```

## Supported Operations

### Fully Supported (Online)
- `ADD INDEX` (with ALGORITHM=INPLACE)
- `ADD COLUMN` (at end, with defaults)
- `DROP INDEX`
- `RENAME TABLE`
- `ADD FOREIGN KEY` (in some cases)

### Requires Caution (Table Copy)
- `CHANGE COLUMN` (type change)
- `MODIFY COLUMN` (type change)
- `ADD COLUMN` (with AFTER clause, no default)
- `ALTER TABLE ... CONVERT TO CHARACTER SET`

### Requires pt-osc (High Risk)
- Large table alterations (>10M rows)
- Operations on tables with triggers
- Complex column modifications

## Configuration

### Connection Strings

```bash
# TCP connection
mysql://user:password@hostname:3306/database

# Unix socket
mysql://user:password@unix(/var/run/mysqld/mysqld.sock)/database

# With SSL
mysql://user:password@hostname:3306/database?tls=true
```

### Environment Variables

```bash
export DATABASE_URL="mysql://root@localhost/mydb"
export DMA_USE_PT_OSC=true
export DMA_PT_OSC_PATH=/usr/local/bin/pt-online-schema-change
```

## Limitations

- MySQL 5.6 and below not supported
- Stored procedures not analyzed
- Trigger modifications not detected
- Partitioning operations need manual review

## Best Practices

1. **Always test migrations** on staging databases first
2. **Use pt-osc** for large tables (>10M rows)
3. **Run during low-traffic windows** for high-risk operations
4. **Monitor replication lag** if using pt-osc
5. **Have rollback plan** ready for each migration

## Troubleshooting

### Connection Issues

```bash
# Test connection
mysql -h hostname -u user -p database -e "SELECT 1"

# Check permissions
GRANT SELECT ON database.* TO 'user'@'hostname';
```

### pt-osc Not Found

```bash
# Install pt-osc
wget percona.com/get/pt-online-schema-change
chmod +x pt-online-schema-change
sudo mv pt-online-schema-change /usr/local/bin/
```

### Parse Errors

If DMA fails to parse your SQL:
1. Validate syntax with `mysql` client
2. Check for unsupported features
3. Report issue with SQL sample

## See Also

- [GitHub Action Usage](github-action-usage.md)
- [GitLab CI Usage](gitlab-ci-usage.md)
- [Phase 3 Changelog](phase3-changelog.md)
