# End-to-End Testing

This directory contains comprehensive end-to-end tests for TAPA that verify the entire system works correctly with real databases.

## Overview

The E2E tests:
- Spin up PostgreSQL and MySQL databases in Docker containers
- Seed them with test data
- Run TAPA analyze command against test migrations
- Verify correct parsing, analysis, and risk scoring
- Test both dry-run mode and database-connected mode
- Validate comprehensive analysis features

## Directory Structure

```
e2e/
├── docker-compose.yml          # Docker Compose configuration
├── run_tests.sh                # Main test runner script
├── postgres/
│   ├── init.sql                # PostgreSQL schema initialization
│   └── seed.sql                # PostgreSQL test data
├── mysql/
│   ├── init.sql                # MySQL schema initialization
│   └── seed.sql                # MySQL test data
├── fixtures/
│   ├── postgres_test_migration.sql  # PostgreSQL test migration
│   └── mysql_test_migration.sql     # MySQL test migration
└── scripts/
    ├── test_postgres.sh        # PostgreSQL-specific tests
    └── test_mysql.sh           # MySQL-specific tests
```

## Prerequisites

- Docker and Docker Compose installed
- Go 1.21+ installed
- `jq` command-line tool for JSON parsing

Install jq on macOS:
```bash
brew install jq
```

Install jq on Linux:
```bash
sudo apt-get install jq  # Debian/Ubuntu
sudo yum install jq      # CentOS/RHEL
```

## Running Tests

### Run Full E2E Test Suite

```bash
cd e2e
./run_tests.sh
```

This will:
1. Build the TAPA binary
2. Start PostgreSQL and MySQL containers
3. Wait for databases to be ready
4. Run PostgreSQL E2E tests
5. Run MySQL E2E tests
6. Clean up containers

### Run Individual Database Tests

**PostgreSQL only:**
```bash
cd e2e
docker-compose up -d postgres
./scripts/test_postgres.sh
docker-compose down
```

**MySQL only:**
```bash
cd e2e
docker-compose up -d mysql
./scripts/test_mysql.sh
docker-compose down
```

## Test Scenarios

### PostgreSQL Tests

1. **Dry-run Analysis** - Tests parsing without database connection
2. **Database Connection Analysis** - Tests with live PostgreSQL connection
3. **Comprehensive Analysis** - Tests enhanced features (dependencies, alternatives)
4. **Operation Detection** - Verifies ADD_COLUMN, CREATE_INDEX detection
5. **Risk Scoring** - Validates risk scores (LOW, MEDIUM, HIGH, CRITICAL)
6. **Recommendations** - Checks that risky operations get recommendations

### MySQL Tests

1. **Dry-run Analysis** - Tests parsing without database connection
2. **Database Connection Analysis** - Tests with live MySQL connection
3. **MySQL-Specific Features** - Verifies ALGORITHM/LOCK clause detection
4. **Operation Detection** - Verifies ADD_COLUMN, MODIFY_COLUMN detection
5. **Risk Scoring** - Validates risk scores
6. **pt-online-schema-change** - Checks pt-osc recommendations for high-risk ops

## Test Database Configuration

### PostgreSQL
- Host: localhost:5433
- User: testuser
- Password: testpass
- Database: testdb

### MySQL
- Host: localhost:3307
- User: testuser
- Password: testpass
- Database: testdb

## Expected Results

All tests should pass with output like:

```
========================================
TAPA End-to-End Test Suite
========================================

[1/6] Building TAPA binary...
✓ TAPA binary built successfully

[2/6] Starting Docker containers...
✓ Docker containers started

[3/6] Waiting for databases to be ready...
  Waiting for PostgreSQL ✓
  Waiting for MySQL ✓

[4/6] Running PostgreSQL E2E test...
Testing PostgreSQL integration...
  Test 1: Dry-run analysis
    ✓ Dry-run analysis successful
  Test 2: Analysis with database connection
    ✓ Database connection analysis successful
  Test 3: Comprehensive analysis
    ✓ Comprehensive analysis successful
  Test 4: Verify operation detection
    ✓ Operations detected correctly: ADD_COLUMN(4), CREATE_INDEX(1)
  Test 5: Verify risk scoring
    ✓ Risk scoring working: MEDIUM(4), HIGH(0)
  Test 6: Verify recommendations
    ✓ Recommendations provided for 4 operations
PostgreSQL E2E tests completed successfully!
✓ PostgreSQL E2E test passed

[5/6] Running MySQL E2E test...
Testing MySQL integration...
  Test 1: Dry-run analysis
    ✓ Dry-run analysis successful
  Test 2: Analysis with database connection
    ✓ Database connection analysis successful
  Test 3: Verify MySQL-specific features
    ⚠ Note: No MySQL metadata detected (ALGORITHM/LOCK clauses)
  Test 4: Verify operation detection
    ✓ Operations detected correctly: ADD_COLUMN(4), MODIFY_COLUMN(0)
  Test 5: Verify risk scoring
    ✓ Risk scoring working: MEDIUM(0), HIGH(2)
  Test 6: Verify pt-osc recommendations
    ⚠ Note: No pt-online-schema-change recommendations
MySQL E2E tests completed successfully!
✓ MySQL E2E test passed

[6/6] Cleaning up...
✓ Cleanup complete

========================================
All E2E tests passed! ✓
========================================
```

## Troubleshooting

### Containers fail to start
```bash
# Check Docker is running
docker ps

# Check for port conflicts
lsof -i :5433  # PostgreSQL
lsof -i :3307  # MySQL

# Clean up existing containers
docker-compose down -v
```

### Database connection timeouts
```bash
# Check database logs
docker logs tapa-e2e-postgres
docker logs tapa-e2e-mysql

# Manually test connections
docker exec tapa-e2e-postgres pg_isready -U testuser -d testdb
docker exec tapa-e2e-mysql mysqladmin ping -h localhost -u testuser -ptestpass
```

### Tests fail to parse JSON
```bash
# Ensure jq is installed
which jq

# Test TAPA output manually
./tapa analyze e2e/fixtures/postgres_test_migration.sql --dry-run --format json
```

## Adding New Tests

To add new E2E tests:

1. Create new migration file in `fixtures/`
2. Update appropriate test script in `scripts/`
3. Add verification assertions
4. Test locally before committing

Example:
```bash
# 1. Create migration
cat > e2e/fixtures/my_test.sql <<EOF
ALTER TABLE users ADD COLUMN phone VARCHAR(20);
EOF

# 2. Add test in scripts/test_postgres.sh
echo -e "${YELLOW}  Test 7: My new test${NC}"
OUTPUT=$("$TAPA_BIN" analyze "$E2E_DIR/fixtures/my_test.sql" --dry-run --format json)
# ... add assertions ...
```

## Continuous Integration

These E2E tests can be integrated into CI/CD pipelines:

```yaml
# .github/workflows/e2e-tests.yml
name: E2E Tests
on: [push, pull_request]
jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Install jq
        run: sudo apt-get install -y jq
      - name: Run E2E tests
        run: cd e2e && ./run_tests.sh
```

## Maintenance

Regular maintenance tasks:

- Update Docker images: `docker-compose pull`
- Clean up volumes: `docker volume prune`
- Update test data as schema evolves
- Add tests for new features
- Keep dependencies up to date
