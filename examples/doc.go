// Package examples holds runnable programs showing how code written against
// pgx is tested with pgxmock. Each subdirectory is a small program together
// with the tests that exercise it against the mock:
//
//   - basic: a transaction updating two tables
//   - blog: an HTTP API server backed by a pool
//   - batch: sending several queries in one pgx.Batch
//
// Package examples holds runnable programs showing how code written against
// pgx is tested with pgxmock. Each subdirectory is a small program together
// with the tests that exercise it against the mock:
//
//   - basic: a transaction updating two tables
//   - blog: an HTTP API server backed by a pool
//   - copyfrom: bulk loading rows with CopyFrom
package examples
