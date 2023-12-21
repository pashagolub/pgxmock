/*
package pgxmock is a mock library implementing pgx connector. Which has one and only
purpose - to simulate pgx driver behavior in tests, without needing a real
database connection. It helps to maintain correct **TDD** workflow.

It does not require (almost) any modifications to your source code in order to test
and mock database operations. Supports concurrency and multiple database mocking.

The driver allows to mock any pgx driver method behavior.
*/
package pgxmock

import (
	"context"
	"fmt"
	"reflect"

	pgx "github.com/jackc/pgx/v5"
	pgconn "github.com/jackc/pgx/v5/pgconn"
	pgxpool "github.com/jackc/pgx/v5/pgxpool"
)

// Expecter interface serves to create expectations
// for any kind of database action in order to mock
// and test real database behavior.
type Expecter interface {
	// ExpectationsWereMet checks whether all queued expectations
	// were met in order (unless MatchExpectationsInOrder set to false).
	// If any of them was not met - an error is returned.
	ExpectationsWereMet() error

	// ExpectClose queues an expectation for this database
	// action to be triggered. The *ExpectedClose allows
	// to mock database response
	ExpectClose() *ExpectedClose

	// ExpectPrepare expects Prepare() to be called with expectedSQL query.
	// the *ExpectedPrepare allows to mock database response.
	// Note that you may expect Query() or Exec() on the *ExpectedPrepare
	// statement to prevent repeating expectedSQL
	ExpectPrepare(expectedStmtName, expectedSQL string) *ExpectedPrepare

	// ExpectQuery expects Query() or QueryRow() to be called with expectedSQL query.
	// the *ExpectedQuery allows to mock database response.
	ExpectQuery(expectedSQL string) *ExpectedQuery

	// ExpectExec expects Exec() to be called with expectedSQL query.
	// the *ExpectedExec allows to mock database response
	ExpectExec(expectedSQL string) *ExpectedExec

	// ExpectBegin expects pgx.Conn.Begin to be called.
	// the *ExpectedBegin allows to mock database response
	ExpectBegin() *ExpectedBegin

	// ExpectBeginTx expects expects BeginTx() to be called with expectedSQL
	// query. The *ExpectedBegin allows to mock database response.
	ExpectBeginTx(txOptions pgx.TxOptions) *ExpectedBegin

	// ExpectCommit expects pgx.Tx.Commit to be called.
	// the *ExpectedCommit allows to mock database response
	ExpectCommit() *ExpectedCommit

	// ExpectReset expects pgxpool.Reset() to be called.
	// The *ExpectedReset allows to mock database response
	ExpectReset() *ExpectedReset

	// ExpectRollback expects pgx.Tx.Rollback to be called.
	// the *ExpectedRollback allows to mock database response
	ExpectRollback() *ExpectedRollback

	// ExpectPing expected Ping() to be called.
	// The *ExpectedPing allows to mock database response
	ExpectPing() *ExpectedPing

	// ExpectCopyFrom expects pgx.CopyFrom to be called.
	// The *ExpectCopyFrom allows to mock database response
	ExpectCopyFrom(expectedTableName pgx.Identifier, expectedColumns []string) *ExpectedCopyFrom

	// MatchExpectationsInOrder gives an option whether to match all
	// expectations in the order they were set or not.
	//
	// By default it is set to - true. But if you use goroutines
	// to parallelize your query executation, that option may
	// be handy.
	//
	// This option may be turned on anytime during tests. As soon
	// as it is switched to false, expectations will be matched
	// in any order. Or otherwise if switched to true, any unmatched
	// expectations will be expected in order
	MatchExpectationsInOrder(bool)

	// NewRows allows Rows to be created from a []string slice.
	NewRows(columns []string) *Rows

	// NewRowsWithColumnDefinition allows Rows to be created from a
	// pgconn.FieldDescription slice with a definition of sql metadata
	NewRowsWithColumnDefinition(columns ...pgconn.FieldDescription) *Rows

	// New Column allows to create a Column
	NewColumn(name string) *pgconn.FieldDescription
}

// PgxCommonIface represents common interface for all pgx connection interfaces:
// pgxpool.Pool, pgx.Conn and pgx.Tx
type PgxCommonIface interface {
	Expecter
	pgx.Tx
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
	Ping(context.Context) error
}

// PgxConnIface represents pgx.Conn specific interface
type PgxConnIface interface {
	PgxCommonIface
	Close(ctx context.Context) error
	Deallocate(ctx context.Context, name string) error
	Config() *pgx.ConnConfig
	PgConn() *pgconn.PgConn
}

// PgxPoolIface represents pgxpool.Pool specific interface
type PgxPoolIface interface {
	PgxCommonIface
	Acquire(ctx context.Context) (*pgxpool.Conn, error)
	AcquireAllIdle(ctx context.Context) []*pgxpool.Conn
	AcquireFunc(ctx context.Context, f func(*pgxpool.Conn) error) error
	AsConn() PgxConnIface
	Close()
	Stat() *pgxpool.Stat
	Reset()
	Config() *pgxpool.Config
}

type pgxmock struct {
	ordered      bool
	queryMatcher QueryMatcher
	expectations []expectation
}

func (c *pgxmock) AcquireAllIdle(_ context.Context) []*pgxpool.Conn {
	return []*pgxpool.Conn{}
}

func (c *pgxmock) AcquireFunc(_ context.Context, _ func(*pgxpool.Conn) error) error {
	return nil
}

// region Expectations
func (c *pgxmock) ExpectClose() *ExpectedClose {
	e := &ExpectedClose{}
	c.expectations = append(c.expectations, e)
	return e
}

func (c *pgxmock) MatchExpectationsInOrder(b bool) {
	c.ordered = b
}

func (c *pgxmock) ExpectationsWereMet() error {
	for _, e := range c.expectations {
		e.Lock()
		fulfilled := e.fulfilled() || !e.required()
		e.Unlock()

		if !fulfilled {
			return fmt.Errorf("there is a remaining expectation which was not matched: %s", e)
		}

		// for expected prepared statement check whether it was closed if expected
		if prep, ok := e.(*ExpectedPrepare); ok {
			if prep.mustBeClosed && !prep.deallocated {
				return fmt.Errorf("expected prepared statement to be closed, but it was not: %s", prep)
			}
		}

		// must check whether all expected queried rows are closed
		if query, ok := e.(*ExpectedQuery); ok {
			if query.rowsMustBeClosed && !query.rowsWereClosed {
				return fmt.Errorf("expected query rows to be closed, but it was not: %s", query)
			}
		}
	}
	return nil
}

func (c *pgxmock) ExpectQuery(expectedSQL string) *ExpectedQuery {
	e := &ExpectedQuery{}
	e.expectSQL = expectedSQL
	c.expectations = append(c.expectations, e)
	return e
}

func (c *pgxmock) ExpectCommit() *ExpectedCommit {
	e := &ExpectedCommit{}
	c.expectations = append(c.expectations, e)
	return e
}

func (c *pgxmock) ExpectRollback() *ExpectedRollback {
	e := &ExpectedRollback{}
	c.expectations = append(c.expectations, e)
	return e
}

func (c *pgxmock) ExpectBegin() *ExpectedBegin {
	e := &ExpectedBegin{}
	c.expectations = append(c.expectations, e)
	return e
}

func (c *pgxmock) ExpectBeginTx(txOptions pgx.TxOptions) *ExpectedBegin {
	e := &ExpectedBegin{opts: txOptions}
	c.expectations = append(c.expectations, e)
	return e
}

func (c *pgxmock) ExpectExec(expectedSQL string) *ExpectedExec {
	e := &ExpectedExec{}
	e.expectSQL = expectedSQL
	c.expectations = append(c.expectations, e)
	return e
}

func (c *pgxmock) ExpectCopyFrom(expectedTableName pgx.Identifier, expectedColumns []string) *ExpectedCopyFrom {
	e := &ExpectedCopyFrom{expectedTableName: expectedTableName, expectedColumns: expectedColumns}
	c.expectations = append(c.expectations, e)
	return e
}

// ExpectReset expects Reset to be called.
func (c *pgxmock) ExpectReset() *ExpectedReset {
	e := &ExpectedReset{}
	c.expectations = append(c.expectations, e)
	return e
}

func (c *pgxmock) ExpectPing() *ExpectedPing {
	e := &ExpectedPing{}
	c.expectations = append(c.expectations, e)
	return e
}

func (c *pgxmock) ExpectPrepare(expectedStmtName, expectedSQL string) *ExpectedPrepare {
	e := &ExpectedPrepare{expectSQL: expectedSQL, expectStmtName: expectedStmtName, mock: c}
	c.expectations = append(c.expectations, e)
	return e
}

//endregion Expectations

// NewRows allows Rows to be created from a
// atring slice or from the CSV string and
// to be used as sql driver.Rows.
func (c *pgxmock) NewRows(columns []string) *Rows {
	r := NewRows(columns)
	return r
}

// PgConn exposes the underlying low level postgres connection
// This is just here to support interfaces that use it. Here is just returns an empty PgConn
func (c *pgxmock) PgConn() *pgconn.PgConn {
	p := pgconn.PgConn{}
	return &p
}

// NewRowsWithColumnDefinition allows Rows to be created from a
// sql driver.Value slice with a definition of sql metadata
func (c *pgxmock) NewRowsWithColumnDefinition(columns ...pgconn.FieldDescription) *Rows {
	r := NewRowsWithColumnDefinition(columns...)
	return r
}

// NewColumn allows to create a Column that can be enhanced with metadata
// using OfType/Nullable/WithLength/WithPrecisionAndScale methods.
func (c *pgxmock) NewColumn(name string) *pgconn.FieldDescription {
	return &pgconn.FieldDescription{Name: name}
}

// open a mock database driver connection
func (c *pgxmock) open(options []func(*pgxmock) error) error {
	for _, option := range options {
		err := option(c)
		if err != nil {
			return err
		}
	}

	if c.queryMatcher == nil {
		c.queryMatcher = QueryMatcherRegexp
	}

	return nil
}

// Close a mock database driver connection. It may or may not
// be called depending on the circumstances, but if it is called
// there must be an *ExpectedClose expectation satisfied.
func (c *pgxmock) Close(ctx context.Context) error {
	ex, err := findExpectation[*ExpectedClose](c, "Close()")
	if err != nil {
		return err
	}
	return ex.waitForDelay(ctx)
}

func (c *pgxmock) Conn() *pgx.Conn {
	panic("Conn() is not available in pgxmock")
}

func (c *pgxmock) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, _ pgx.CopyFromSource) (int64, error) {
	ex, err := findExpectationFunc[*ExpectedCopyFrom](c, "BeginTx()", func(copyExp *ExpectedCopyFrom) error {
		if !reflect.DeepEqual(copyExp.expectedTableName, tableName) {
			return fmt.Errorf("CopyFrom: table name '%s' was not expected, expected table name is '%s'", tableName, copyExp.expectedTableName)
		}
		if !reflect.DeepEqual(copyExp.expectedColumns, columnNames) {
			return fmt.Errorf("CopyFrom: column names '%v' were not expected, expected column names are '%v'", columnNames, copyExp.expectedColumns)
		}
		return nil
	})
	if err != nil {
		return -1, err
	}
	return ex.rowsAffected, ex.waitForDelay(ctx)
}

func (c *pgxmock) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (c *pgxmock) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (c *pgxmock) Begin(ctx context.Context) (pgx.Tx, error) {
	return c.BeginTx(ctx, pgx.TxOptions{})
}

func (c *pgxmock) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	ex, err := findExpectationFunc[*ExpectedBegin](c, "BeginTx()", func(beginExp *ExpectedBegin) error {
		if beginExp.opts != txOptions {
			return fmt.Errorf("BeginTx: call with transaction options '%v' was not expected: %s", txOptions, beginExp)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err = ex.waitForDelay(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *pgxmock) Prepare(ctx context.Context, name, query string) (*pgconn.StatementDescription, error) {
	ex, err := findExpectationFunc[*ExpectedPrepare](c, "Prepare()", func(prepareExp *ExpectedPrepare) error {
		if err := c.queryMatcher.Match(prepareExp.expectSQL, query); err != nil {
			return err
		}
		if prepareExp.expectStmtName != name {
			return fmt.Errorf("Prepare: prepared statement name '%s' was not expected, expected name is '%s'", name, prepareExp.expectStmtName)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err = ex.waitForDelay(ctx); err != nil {
		return nil, err
	}
	return &pgconn.StatementDescription{Name: name, SQL: query}, nil
}

func (c *pgxmock) Deallocate(ctx context.Context, name string) error {
	var (
		expected *ExpectedPrepare
		ok       bool
	)
	for _, next := range c.expectations {
		next.Lock()
		expected, ok = next.(*ExpectedPrepare)
		ok = ok && expected.expectStmtName == name
		next.Unlock()
		if ok {
			break
		}
	}
	if expected == nil {
		return fmt.Errorf("Deallocate: prepared statement name '%s' doesn't exist", name)
	}
	expected.deallocated = true
	return expected.waitForDelay(ctx)
}

func (c *pgxmock) Commit(ctx context.Context) error {
	ex, err := findExpectation[*ExpectedCommit](c, "Commit()")
	if err != nil {
		return err
	}
	return ex.waitForDelay(ctx)
}

func (c *pgxmock) Rollback(ctx context.Context) error {
	ex, err := findExpectation[*ExpectedRollback](c, "Rollback()")
	if err != nil {
		return err
	}
	return ex.waitForDelay(ctx)
}

// Implement the "QueryerContext" interface
func (c *pgxmock) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	ex, err := findExpectationFunc[*ExpectedQuery](c, "Query()", func(queryExp *ExpectedQuery) error {
		if err := c.queryMatcher.Match(queryExp.expectSQL, sql); err != nil {
			return err
		}
		if rewrittenSQL, err := queryExp.argsMatches(sql, args); err != nil {
			return err
		} else if rewrittenSQL != "" && queryExp.expectRewrittenSQL != "" {
			if err := c.queryMatcher.Match(queryExp.expectRewrittenSQL, rewrittenSQL); err != nil {
				return err
			}
		}
		if queryExp.err == nil && queryExp.rows == nil {
			return fmt.Errorf("Query must return a result rows or raise an error: %v", queryExp)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ex.rows, ex.waitForDelay(ctx)
}

type errRow struct {
	err error
}

func (er errRow) Scan(...interface{}) error {
	return er.err
}

func (c *pgxmock) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	rows, err := c.Query(ctx, sql, args...)
	if err != nil {
		return errRow{err}
	}
	_ = rows.Next()
	return rows
}

func (c *pgxmock) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	ex, err := findExpectationFunc[*ExpectedExec](c, "Exec()", func(execExp *ExpectedExec) error {
		if err := c.queryMatcher.Match(execExp.expectSQL, query); err != nil {
			return err
		}
		if rewrittenSQL, err := execExp.argsMatches(query, args); err != nil {
			return err
		} else if rewrittenSQL != "" && execExp.expectRewrittenSQL != "" {
			if err := c.queryMatcher.Match(execExp.expectRewrittenSQL, rewrittenSQL); err != nil {
				return err
			}
		}
		if execExp.result.String() == "" && execExp.err == nil {
			return fmt.Errorf("Exec must return a result or raise an error: %s", execExp)
		}
		return nil
	})
	if err != nil {
		return pgconn.NewCommandTag(""), err
	}
	return ex.result, ex.waitForDelay(ctx)
}

func (c *pgxmock) Ping(ctx context.Context) (err error) {
	ex, err := findExpectation[*ExpectedPing](c, "Ping()")
	if err != nil {
		return err
	}
	return ex.waitForDelay(ctx)
}

func (c *pgxmock) Reset() {
	ex, err := findExpectation[*ExpectedReset](c, "Reset()")
	if err != nil {
		return
	}
	_ = ex.waitForDelay(context.Background())
}

type expectationType[t any] interface {
	*t
	expectation
}

func findExpectationFunc[ET expectationType[t], t any](c *pgxmock, method string, cmp func(ET) error) (ET, error) {
	var expected ET
	var fulfilled int
	var ok bool
	var err error
	for _, next := range c.expectations {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if expected, ok = next.(ET); ok {
			err = cmp(expected)
			if err == nil {
				break
			}
		}
		if c.ordered {
			if (!ok || err != nil) && !next.required() {
				next.Unlock()
				continue
			}
			next.Unlock()
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("call to method %s, was not expected, next expectation is: %s", method, next)
		}
		next.Unlock()
	}

	if expected == nil {
		msg := fmt.Sprintf("call to method %s was not expected", method)
		if fulfilled == len(c.expectations) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return nil, fmt.Errorf(msg)
	}
	defer expected.Unlock()

	expected.fulfill()
	return expected, nil
}

func findExpectation[ET expectationType[t], t any](c *pgxmock, method string) (ET, error) {
	return findExpectationFunc[ET, t](c, method, func(_ ET) error { return nil })
}
