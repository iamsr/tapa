# Integration Tests

Integration tests are co-located with the code they test, following Go conventions.

## Location

Integration tests are in the same package as the code:
- `internal/analyzer/postgres/integration_test.go`
- `internal/analyzer/mysql/integration_test.go`

## Running Integration Tests

```bash
# PostgreSQL integration tests
go test ./internal/analyzer/postgres -v -run Integration

# MySQL integration tests
go test ./internal/analyzer/mysql -v -run Integration

# All integration tests (requires Docker)
cd tests/e2e && docker-compose up -d
go test ./internal/analyzer/... -v -run Integration
cd tests/e2e && docker-compose down -v
```

## Why Co-located?

Integration tests need access to package-private functions and test helpers,
so they live alongside the code. This follows Go best practices.

Unit tests for utilities that don't require database access can be found
in `tests/unit/`.
