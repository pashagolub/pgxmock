# AGENTS.md

pgxmock mocks pgx v5 (`*pgx.Conn`, `*pgxpool.Pool`, `pgx.Tx`) so code can be tested without a database.

## Rules

- **pgx parity**: the mock behaves exactly like pgx wherever pgx defines the behaviour. When they disagree, change the mock, even if that breaks tests written against it.
- Every `Expected*` type has its own `Maybe`, `Times`, `WillDelayFor`, `WillReturnError` and `WillPanic` in `modifiers.go`, each returning the concrete type so chains work in any order. A new expectation type gets its own section there.
- Docs and examples have the code under test accept a small interface of its own that `*pgx.Conn` or `*pgxpool.Pool` satisfies, keeping pgxmock a test-only import.
- Each godoc example tests code under test: it lives alone in its own `example_*_test.go`, next to the function it calls and that function's small interface, so pkg.go.dev shows the whole file.
- Each test guards behaviour: it fails without the change it ships with. Wording changes to error and panic messages ship on their own.
- Comments are one-liners matching the density of the file; rationale goes in the commit message.

## Commits

The subject, and the PR title, starts with a prefix: `[+]` feature, `[-]` fix, `[*]` change, `[!]` breaking change or breakthrough functionality.

## Commands

As CI runs them:

```bash
go test -race ./...
go test ./... -coverprofile=coverage.out -coverpkg=github.com/pashagolub/pgxmock/v6  # examples count toward coverage
golangci-lint run ./...
```
