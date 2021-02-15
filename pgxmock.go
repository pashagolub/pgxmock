/*
package pgxmock is a mock library implementing sql driver. Which has one and only
purpose - to simulate any sql driver behavior in tests, without needing a real
database connection. It helps to maintain correct **TDD** workflow.

It does not require any modifications to your source code in order to test
and mock database operations. Supports concurrency and multiple database mocking.

The driver allows to mock any sql driver method behavior.
*/
package pgxmock

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgx/v4"
)

// Sqlmock interface serves to create expectations
// for any kind of database action in order to mock
// and test real database behavior.
type pgxMockIface interface {
	// ExpectClose queues an expectation for this database
	// action to be triggered. the *ExpectedClose allows
	// to mock database response
	ExpectClose() *ExpectedClose

	// ExpectationsWereMet checks whether all queued expectations
	// were met in order. If any of them was not met - an error is returned.
	ExpectationsWereMet() error

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

	// ExpectBegin expects *sql.DB.Begin to be called.
	// the *ExpectedBegin allows to mock database response
	ExpectBegin() *ExpectedBegin

	// ExpectCommit expects *sql.Tx.Commit to be called.
	// the *ExpectedCommit allows to mock database response
	ExpectCommit() *ExpectedCommit

	// ExpectRollback expects *sql.Tx.Rollback to be called.
	// the *ExpectedRollback allows to mock database response
	ExpectRollback() *ExpectedRollback

	// ExpectPing expected *sql.DB.Ping to be called.
	// the *ExpectedPing allows to mock database response
	//
	// Ping support only exists in the SQL library in Go 1.8 and above.
	// ExpectPing in Go <=1.7 will return an ExpectedPing but not register
	// any expectations.
	//
	// You must enable pings using MonitorPingsOption for this to register
	// any expectations.
	ExpectPing() *ExpectedPing

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

	// NewRows allows Rows to be created from a
	// sql driver.Value slice or from the CSV string and
	// to be used as sql driver.Rows.
	NewRows(columns []string) *Rows

	// NewRowsWithColumnDefinition allows Rows to be created from a
	// sql driver.Value slice with a definition of sql metadata
	NewRowsWithColumnDefinition(columns ...pgproto3.FieldDescription) *Rows

	// New Column allows to create a Column
	NewColumn(name string) *pgproto3.FieldDescription
}

type pgxIface interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	Ping(context.Context) error
	Prepare(context.Context, string, string) (*pgconn.StatementDescription, error)
	Deallocate(ctx context.Context, name string) error
	Close(context.Context) error
}

type Pgxmock interface {
	pgxIface
	pgxMockIface
	pgx.Tx
}

type pgxmock struct {
	ordered bool
	dsn     string
	// opened  int
	// converter    driver.ValueConverter
	queryMatcher QueryMatcher
	monitorPings bool

	expected []expectation
}

func (c *pgxmock) open(options []func(*pgxmock) error) (Pgxmock, error) {
	for _, option := range options {
		err := option(c)
		if err != nil {
			return c, err
		}
	}
	// if c.converter == nil {
	// 	c.converter = driver.DefaultParameterConverter
	// }
	if c.queryMatcher == nil {
		c.queryMatcher = QueryMatcherRegexp
	}

	if c.monitorPings {
		// We call Ping on the driver shortly to verify startup assertions by
		// driving internal behaviour of the sql standard library. We don't
		// want this call to ping to be monitored for expectation purposes so
		// temporarily disable.
		c.monitorPings = false
		defer func() { c.monitorPings = true }()
	}
	return c, c.Ping(context.TODO())
}

func (c *pgxmock) ExpectClose() *ExpectedClose {
	e := &ExpectedClose{}
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) MatchExpectationsInOrder(b bool) {
	c.ordered = b
}

// Close a mock database driver connection. It may or may not
// be called depending on the circumstances, but if it is called
// there must be an *ExpectedClose expectation satisfied.
func (c *pgxmock) Close(context.Context) error {
	var expected *ExpectedClose
	var fulfilled int
	var ok bool
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if expected, ok = next.(*ExpectedClose); ok {
			break
		}

		next.Unlock()
		if c.ordered {
			return fmt.Errorf("call to database Close, was not expected, next expectation is: %s", next)
		}
	}

	if expected == nil {
		msg := "call to database Close was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return fmt.Errorf(msg)
	}

	expected.triggered = true
	expected.Unlock()
	return expected.err
}

func (c *pgxmock) ExpectationsWereMet() error {
	for _, e := range c.expected {
		e.Lock()
		fulfilled := e.fulfilled()
		e.Unlock()

		if !fulfilled {
			return fmt.Errorf("there is a remaining expectation which was not matched: %s", e)
		}

		// for expected prepared statement check whether it was closed if expected
		if prep, ok := e.(*ExpectedPrepare); ok {
			if prep.mustBeClosed && !prep.wasClosed {
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

func (c *pgxmock) Conn() *pgx.Conn {
	return nil
}

func (c *pgxmock) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (c *pgxmock) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return nil
}

func (c *pgxmock) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (c *pgxmock) QueryFunc(ctx context.Context, sql string, args []interface{}, scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	return nil, nil
}

func (c *pgxmock) Begin(ctx context.Context) (pgx.Tx, error) {
	ex, err := c.begin()
	if ex != nil {
		time.Sleep(ex.delay)
	}
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *pgxmock) begin() (*ExpectedBegin, error) {
	var expected *ExpectedBegin
	var ok bool
	var fulfilled int
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if expected, ok = next.(*ExpectedBegin); ok {
			break
		}

		next.Unlock()
		if c.ordered {
			return nil, fmt.Errorf("call to database transaction Begin, was not expected, next expectation is: %s", next)
		}
	}
	if expected == nil {
		msg := "call to database transaction Begin was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return nil, fmt.Errorf(msg)
	}

	expected.triggered = true
	expected.Unlock()

	return expected, expected.err
}

func (c *pgxmock) ExpectBegin() *ExpectedBegin {
	e := &ExpectedBegin{}
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) ExpectExec(expectedSQL string) *ExpectedExec {
	e := &ExpectedExec{}
	e.expectSQL = expectedSQL
	// e.converter = c.converter
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) Prepare(ctx context.Context, name, query string) (*pgconn.StatementDescription, error) {
	ex, err := c.prepare(name, query)
	if ex != nil {
		time.Sleep(ex.delay)
	}
	if err != nil {
		return nil, err
	}

	return &pgconn.StatementDescription{Name: name, SQL: query}, nil
}

func (c *pgxmock) prepare(name string, query string) (*ExpectedPrepare, error) {
	var expected *ExpectedPrepare
	var fulfilled int
	var ok bool

	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if c.ordered {
			if expected, ok = next.(*ExpectedPrepare); ok {
				break
			}

			next.Unlock()
			return nil, fmt.Errorf("call to Prepare statement with query '%s', was not expected, next expectation is: %s", query, next)
		}

		if pr, ok := next.(*ExpectedPrepare); ok {
			if err := c.queryMatcher.Match(pr.expectSQL, query); err == nil {
				expected = pr
				break
			}
		}
		next.Unlock()
	}

	if expected == nil {
		msg := "call to Prepare '%s' query was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return nil, fmt.Errorf(msg, query)
	}
	defer expected.Unlock()
	if expected.expectStmtName != name {
		return nil, fmt.Errorf("Prepare: prepared statement name '%s' was not expected, expected name is '%s'", name, expected.expectStmtName)
	}
	if err := c.queryMatcher.Match(expected.expectSQL, query); err != nil {
		return nil, fmt.Errorf("Prepare: %v", err)
	}

	expected.triggered = true
	return expected, expected.err
}

func (c *pgxmock) ExpectPrepare(expectedStmtName, expectedSQL string) *ExpectedPrepare {
	e := &ExpectedPrepare{expectSQL: expectedSQL, expectStmtName: expectedStmtName, mock: c}
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) Deallocate(ctx context.Context, name string) error {
	var expected *ExpectedPrepare
	for _, next := range c.expected {
		next.Lock()
		if pr, ok := next.(*ExpectedPrepare); ok && pr.expectStmtName == name {
			expected = pr
			next.Unlock()
			break
		}
		next.Unlock()
	}
	if expected == nil {
		return fmt.Errorf("Deallocate: prepared statement name '%s' doesn't exist", name)
	}
	expected.wasClosed = true
	return nil
}

func (c *pgxmock) ExpectQuery(expectedSQL string) *ExpectedQuery {
	e := &ExpectedQuery{}
	e.expectSQL = expectedSQL
	// e.converter = c.converter
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) ExpectCommit() *ExpectedCommit {
	e := &ExpectedCommit{}
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) ExpectRollback() *ExpectedRollback {
	e := &ExpectedRollback{}
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) Commit(ctx context.Context) error {
	var expected *ExpectedCommit
	var fulfilled int
	var ok bool
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if expected, ok = next.(*ExpectedCommit); ok {
			break
		}

		next.Unlock()
		if c.ordered {
			return fmt.Errorf("call to Commit transaction, was not expected, next expectation is: %s", next)
		}
	}
	if expected == nil {
		msg := "call to Commit transaction was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return fmt.Errorf(msg)
	}

	expected.triggered = true
	expected.Unlock()
	return expected.err
}

func (c *pgxmock) Rollback(ctx context.Context) error {
	var expected *ExpectedRollback
	var fulfilled int
	var ok bool
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if expected, ok = next.(*ExpectedRollback); ok {
			break
		}

		next.Unlock()
		if c.ordered {
			return fmt.Errorf("call to Rollback transaction, was not expected, next expectation is: %s", next)
		}
	}
	if expected == nil {
		msg := "call to Rollback transaction was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return fmt.Errorf(msg)
	}

	expected.triggered = true
	expected.Unlock()
	return expected.err
}

// NewRows allows Rows to be created from a
// sql driver.Value slice or from the CSV string and
// to be used as sql driver.Rows.
func (c *pgxmock) NewRows(columns []string) *Rows {
	r := NewRows(columns)
	// r.converter = c.converter
	return r
}

// ErrCancelled defines an error value, which can be expected in case of
// such cancellation error.
var ErrCancelled = errors.New("canceling query due to user request")

// Implement the "QueryerContext" interface
func (c *pgxmock) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	ex, err := c.query(sql, args)
	if ex != nil {
		select {
		case <-time.After(ex.delay):
			if err != nil {
				return nil, err
			}
			return ex.rows, nil
		case <-ctx.Done():
			return nil, ErrCancelled
		}
	}

	return nil, err
}

func (c *pgxmock) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	ex, err := c.query(sql, args)
	if ex != nil {
		select {
		case <-time.After(ex.delay):
			if (err != nil) || (ex.rows == nil) {
				return nil
			}
			_ = ex.rows.Next()
			return ex.rows
		case <-ctx.Done():
			return nil
		}
	}

	return nil
}

// Implement the "ExecerContext" interface
func (c *pgxmock) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	ex, err := c.exec(query, args)
	if ex != nil {
		select {
		case <-time.After(ex.delay):
			if err != nil {
				return nil, err
			}
			return ex.result, nil
		case <-ctx.Done():
			return nil, ErrCancelled
		}
	}

	return nil, err
}

// Implement the "Pinger" interface - the explicit DB driver ping was only added to database/sql in Go 1.8
func (c *pgxmock) Ping(ctx context.Context) error {
	if !c.monitorPings {
		return nil
	}

	ex, err := c.ping()
	if ex != nil {
		select {
		case <-ctx.Done():
			return ErrCancelled
		case <-time.After(ex.delay):
		}
	}

	return err
}

func (c *pgxmock) ping() (*ExpectedPing, error) {
	var expected *ExpectedPing
	var fulfilled int
	var ok bool
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if expected, ok = next.(*ExpectedPing); ok {
			break
		}

		next.Unlock()
		if c.ordered {
			return nil, fmt.Errorf("call to database Ping, was not expected, next expectation is: %s", next)
		}
	}

	if expected == nil {
		msg := "call to database Ping was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return nil, fmt.Errorf(msg)
	}

	expected.triggered = true
	expected.Unlock()
	return expected, expected.err
}

func (c *pgxmock) ExpectPing() *ExpectedPing {
	if !c.monitorPings {
		log.Println("ExpectPing will have no effect as monitoring pings is disabled. Use MonitorPingsOption to enable.")
		return nil
	}
	e := &ExpectedPing{}
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) query(query string, args []interface{}) (*ExpectedQuery, error) {
	var expected *ExpectedQuery
	var fulfilled int
	var ok bool
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if c.ordered {
			if expected, ok = next.(*ExpectedQuery); ok {
				break
			}
			next.Unlock()
			return nil, fmt.Errorf("call to Query '%s' with args %+v, was not expected, next expectation is: %s", query, args, next)
		}
		if qr, ok := next.(*ExpectedQuery); ok {
			if err := c.queryMatcher.Match(qr.expectSQL, query); err != nil {
				next.Unlock()
				continue
			}
			if err := qr.attemptArgMatch(args); err == nil {
				expected = qr
				break
			}
		}
		next.Unlock()
	}

	if expected == nil {
		msg := "call to Query '%s' with args %+v was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return nil, fmt.Errorf(msg, query, args)
	}

	defer expected.Unlock()

	if err := c.queryMatcher.Match(expected.expectSQL, query); err != nil {
		return nil, fmt.Errorf("Query: %v", err)
	}

	if err := expected.argsMatches(args); err != nil {
		return nil, fmt.Errorf("Query '%s', arguments do not match: %s", query, err)
	}

	expected.triggered = true
	if expected.err != nil {
		return expected, expected.err // mocked to return error
	}

	if expected.rows == nil {
		return nil, fmt.Errorf("Query '%s' with args %+v, must return a pgx.Rows, but it was not set for expectation %T as %+v", query, args, expected, expected)
	}
	return expected, nil
}

func (c *pgxmock) exec(query string, args []interface{}) (*ExpectedExec, error) {
	var expected *ExpectedExec
	var fulfilled int
	var ok bool
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if c.ordered {
			if expected, ok = next.(*ExpectedExec); ok {
				break
			}
			next.Unlock()
			return nil, fmt.Errorf("call to ExecQuery '%s' with args %+v, was not expected, next expectation is: %s", query, args, next)
		}
		if exec, ok := next.(*ExpectedExec); ok {
			if err := c.queryMatcher.Match(exec.expectSQL, query); err != nil {
				next.Unlock()
				continue
			}

			if err := exec.attemptArgMatch(args); err == nil {
				expected = exec
				break
			}
		}
		next.Unlock()
	}
	if expected == nil {
		msg := "call to ExecQuery '%s' with args %+v was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return nil, fmt.Errorf(msg, query, args)
	}
	defer expected.Unlock()

	if err := c.queryMatcher.Match(expected.expectSQL, query); err != nil {
		return nil, fmt.Errorf("ExecQuery: %v", err)
	}

	if err := expected.argsMatches(args); err != nil {
		return nil, fmt.Errorf("ExecQuery '%s', arguments do not match: %s", query, err)
	}

	expected.triggered = true
	if expected.err != nil {
		return expected, expected.err // mocked to return error
	}

	if expected.result == nil {
		return nil, fmt.Errorf("ExecQuery '%s' with args %+v, must return a pgconn.CommandTag, but it was not set for expectation %T as %+v", query, args, expected, expected)
	}

	return expected, nil
}

// @TODO maybe add ExpectedBegin.WithOptions(driver.TxOptions)

// NewRowsWithColumnDefinition allows Rows to be created from a
// sql driver.Value slice with a definition of sql metadata
func (c *pgxmock) NewRowsWithColumnDefinition(columns ...pgproto3.FieldDescription) *Rows {
	r := NewRowsWithColumnDefinition(columns...)
	// r.converter = c.converter
	return r
}

// NewColumn allows to create a Column that can be enhanced with metadata
// using OfType/Nullable/WithLength/WithPrecisionAndScale methods.
func (c *pgxmock) NewColumn(name string) *pgproto3.FieldDescription {
	return &pgproto3.FieldDescription{Name: []byte(name)}
}
