# pgxmock AI Coding Instructions

## Project Overview
pgxmock is a mock library for the pgx PostgreSQL driver, enabling database testing without real connections. It follows a strict expectation-driven testing pattern inspired by sqlmock but specifically designed for pgx interfaces.

## Core Architecture

### Interface Hierarchy
- **`Expecter`**: Main interface for setting up database expectations (`pgxmock.go:27`)
- **`PgxCommonIface`**: Base interface for all pgx connections (Conn/Pool agnostic)
- **`PgxConnIface`**: Extends PgxCommonIface for single connections
- **`PgxPoolIface`**: Extends PgxCommonIface for connection pools

### Key Components
- **Expectations**: Each database operation (Query, Exec, Begin, etc.) has a corresponding `Expected*` type
- **Query Matching**: Configurable via `QueryMatcher` interface (default: regex-based)
- **Row Mocking**: Custom `Rows` type that simulates `pgx.Rows` behavior

## Essential Testing Patterns

### Mock Creation
```go
// For connection pools (most common)
mock, err := pgxmock.NewPool()

// For single connections  
mock, err := pgxmock.NewConn()

// Always defer cleanup
defer mock.Close()
```

### Expectation Chain Pattern
Every test follows this structure:
1. Set expectations in execution order
2. Execute the code under test
3. Assert all expectations were met with `mock.ExpectationsWereMet()`

### Common Expectation Patterns
```go
// Transaction workflow
mock.ExpectBegin()
mock.ExpectExec("UPDATE products").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
mock.ExpectExec("INSERT INTO product_viewers").WithArgs(2, 3).WillReturnResult(pgxmock.NewResult("INSERT", 1))
mock.ExpectCommit() // or ExpectRollback() for error cases

// Query with rows
mock.ExpectQuery("SELECT (.+) FROM articles").WithArgs(5).WillReturnRows(rows)
```

## Development Conventions

### File Organization
- **Core**: `pgxmock.go`, `expectations.go` - main interfaces and expectation logic
- **Operations**: `query.go`, `batch.go`, `rows.go` - specific operation implementations  
- **Tests**: `*_test.go` files demonstrate patterns, especially `pgxmock_test.go`
- **Examples**: `examples/` directory shows real-world usage patterns

### Error Handling
- Use `WillReturnError()` to simulate database errors
- Always test both success and failure paths
- Error expectations must match the actual error returned

### Query Matching
- Default: Regex-based matching via `QueryMatcherRegexp`
- Alternative: Exact matching via `QueryMatcherEqual`
- Custom matchers: Implement `QueryMatcher` interface
- SQL is automatically stripped of extra whitespace

### Argument Matching
- Use `WithArgs()` for exact parameter matching
- Implement `Argument` interface for complex types (e.g., `time.Time`)
- Arguments must match in exact order and type

## Testing Workflow

### Build & Test
```bash
# Run tests with race detection
go test -race

# Run with coverage and parsing
go test -failfast -p 1 -timeout=300s -parallel=1 ./... -coverprofile='coverage.out' -json | tparse -all -progress

# Generate coverage report
go tool cover -func='coverage.out' && go tool cover -html='coverage.out'
```

### VS Code Tasks
- **Unit Test**: Runs tests with coverage and formatted output
- **Coverage Report**: Generates and opens HTML coverage report
- **Lint**: Runs golangci-lint for code quality

## Critical Implementation Details

### Interface Compatibility
When mocking code, ensure your functions accept interfaces rather than concrete types:
```go
// Good - testable
func RecordStats(db PgxIface, userID, productID int) error

// Bad - not mockable  
func RecordStats(db *pgx.Conn, userID, productID int) error
```

### Expectation Order
- By default, expectations must be called in the exact order they were set
- Use `MatchExpectationsInOrder(false)` for parallel/unordered execution
- Each expectation can be made optional with `.Maybe()`

### Common Pitfalls
- Forgetting to call `ExpectationsWereMet()` in tests
- SQL regex patterns that are too strict or too loose
- Mismatched argument types in `WithArgs()`
- Not handling both transaction commit and rollback scenarios

## Integration Points
- **pgx v5**: Direct compatibility with latest pgx interfaces
- **testify**: Used in test suite for assertions (`assert` package)
- **Examples**: `examples/basic/` and `examples/blog/` show real-world integration patterns
